package call

import (
	"context"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/events"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/idgen"
)

type CreateInput struct {
	AppID        string
	CallerID     string
	CallerDevice string
	CalleeIDs    []string
	Type         string
	Mode         string
	CallerName   string
}

type Service struct {
	store         Store
	bus           *events.Bus
	ringTimeout   time.Duration
	onRingTimeout func(callID string)
}

func NewService(store Store, bus *events.Bus, ringTimeout time.Duration) *Service {
	if ringTimeout <= 0 {
		ringTimeout = 45 * time.Second
	}
	return &Service{store: store, bus: bus, ringTimeout: ringTimeout}
}

func (s *Service) SetRingTimeoutHandler(fn func(callID string)) {
	s.onRingTimeout = fn
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Call, error) {
	if in.CallerID == "" || len(in.CalleeIDs) == 0 {
		return nil, domain.InvalidRequest("caller_id and callees are required")
	}
	typ := in.Type
	if typ == "" {
		typ = TypeAudio
	}
	if typ != TypeAudio && typ != TypeVideo {
		return nil, domain.InvalidRequest("type must be audio or video")
	}
	mode := in.Mode
	if mode == "" {
		if len(in.CalleeIDs) == 1 {
			mode = ModeDirect
		} else {
			mode = ModeGroup
		}
	}
	if mode != ModeDirect && mode != ModeGroup {
		return nil, domain.InvalidRequest("mode must be direct or group")
	}
	if mode == ModeDirect && len(in.CalleeIDs) != 1 {
		return nil, domain.InvalidRequest("direct call requires exactly one callee")
	}
	transport := TransportP2P
	if mode == ModeGroup {
		transport = TransportSFU
	}

	now := time.Now().UTC()
	callID := "call_" + idgen.New()
	roomID := "room_" + idgen.New()

	participants := []*Participant{
		{
			CallID:   callID,
			UserID:   in.CallerID,
			DeviceID: in.CallerDevice,
			State:    ParticipantAccepted,
			JoinedAt: &now,
			Role:     "caller",
		},
	}
	for _, uid := range in.CalleeIDs {
		if uid == in.CallerID {
			continue
		}
		participants = append(participants, &Participant{
			CallID: callID,
			UserID: uid,
			State:  ParticipantRinging,
			Role:   "callee",
		})
	}

	c := &Call{
		ID:           callID,
		RoomID:       roomID,
		AppID:        in.AppID,
		CallerID:     in.CallerID,
		Type:         typ,
		Mode:         mode,
		State:        StateRinging,
		Transport:    transport,
		CreatedAt:    now,
		Participants: participants,
	}

	if err := s.store.Put(ctx, c); err != nil {
		return nil, err
	}

	s.publish(events.CallCreated, c, in.CallerID, nil)
	s.publish(events.CallRinging, c, in.CallerID, nil)

	go s.scheduleTimeout(callID)

	return c.Clone(), nil
}

func (s *Service) scheduleTimeout(callID string) {
	timer := time.NewTimer(s.ringTimeout)
	defer timer.Stop()
	<-timer.C

	// 注意：后台定时器任务必须使用 context.Background()，不能复用请求 ctx
	ctx := context.Background()
	_, err := s.store.Update(ctx, callID, func(c *Call) error {
		if c.State != StateRinging {
			return errSkip
		}
		now := time.Now().UTC()
		c.State = StateTimeout
		c.EndedAt = &now
		c.Reason = "ring_timeout"
		for _, p := range c.Participants {
			if p.State == ParticipantRinging || p.State == ParticipantInvited {
				p.State = ParticipantDisconnected
				p.LeftAt = &now
			}
		}
		return nil
	})
	// errSkip：通话已离开 ringing，无需发 timeout 事件；其它错误同样中止
	if err != nil {
		return
	}

	if c, err := s.store.Get(ctx, callID); err == nil {
		s.publish(events.CallEnded, c, "", map[string]any{"reason": "ring_timeout"})
	}
	if s.onRingTimeout != nil {
		s.onRingTimeout(callID)
	}
}

var errSkip = domain.InvalidRequest("skip")

func (s *Service) Get(ctx context.Context, callID string) (*Call, error) {
	return s.store.Get(ctx, callID)
}

func (s *Service) Accept(ctx context.Context, callID, userID, deviceID string) (*Call, error) {
	c, err := s.store.Update(ctx, callID, func(c *Call) error {
		if c.IsTerminal() {
			return domain.InvalidRequest("call already ended")
		}
		if c.State != StateRinging && c.State != StateAccepted && c.State != StateConnecting {
			return domain.InvalidRequest("call not ringing")
		}
		p := c.FindParticipant(userID)
		if p == nil {
			return domain.Forbidden("not a participant")
		}

		now := time.Now().UTC()
		p.State = ParticipantAccepted
		p.DeviceID = deviceID
		p.JoinedAt = &now
		c.State = StateAccepted

		if c.Mode == ModeDirect {
			for _, other := range c.Participants {
				if other.UserID != userID && other.UserID != c.CallerID &&
					(other.State == ParticipantRinging || other.State == ParticipantInvited) {
					other.State = ParticipantDisconnected
					other.LeftAt = &now
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	s.publish(events.CallAccepted, c, userID, map[string]any{
		"accepted_device_id": deviceID,
		"stop_ringing":       true,
	})
	return c, nil
}

func (s *Service) Reject(ctx context.Context, callID, userID string) (*Call, error) {
	c, err := s.store.Update(ctx, callID, func(c *Call) error {
		if c.IsTerminal() {
			return domain.InvalidRequest("call already ended")
		}
		p := c.FindParticipant(userID)
		if p == nil {
			return domain.Forbidden("not a participant")
		}

		now := time.Now().UTC()
		p.State = ParticipantRejected
		p.LeftAt = &now

		if c.Mode == ModeDirect {
			c.State = StateRejected
			c.EndedAt = &now
			c.Reason = "rejected"
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	s.publish(events.CallRejected, c, userID, nil)
	if c.State == StateRejected {
		s.publish(events.CallEnded, c, userID, map[string]any{"reason": "rejected"})
	}
	return c, nil
}

func (s *Service) Cancel(ctx context.Context, callID, userID string) (*Call, error) {
	c, err := s.store.Update(ctx, callID, func(c *Call) error {
		if c.IsTerminal() {
			return domain.InvalidRequest("call already ended")
		}
		if c.CallerID != userID {
			return domain.Forbidden("only caller can cancel")
		}

		now := time.Now().UTC()
		c.State = StateCancelled
		c.EndedAt = &now
		c.Reason = "cancelled"

		for _, p := range c.Participants {
			if p.State == ParticipantRinging || p.State == ParticipantInvited {
				p.State = ParticipantDisconnected
				p.LeftAt = &now
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	s.publish(events.CallCancelled, c, userID, nil)
	s.publish(events.CallEnded, c, userID, map[string]any{"reason": "cancelled"})
	return c, nil
}

func (s *Service) Join(ctx context.Context, callID, userID, deviceID string) (*Call, error) {
	c, err := s.store.Update(ctx, callID, func(c *Call) error {
		if c.IsTerminal() {
			return domain.InvalidRequest("call already ended")
		}
		p := c.FindParticipant(userID)
		now := time.Now().UTC()

		if p == nil {
			if c.Mode != ModeGroup {
				return domain.Forbidden("not a participant")
			}
			p = &Participant{CallID: callID, UserID: userID, Role: "participant"}
			c.Participants = append(c.Participants, p)
		}
		p.DeviceID = deviceID
		p.State = ParticipantConnecting
		if p.JoinedAt == nil {
			p.JoinedAt = &now
		}

		if c.State == StateAccepted || c.State == StateRinging {
			c.State = StateConnecting
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	s.publish(events.CallParticipantJoined, c, userID, nil)
	return c, nil
}

func (s *Service) MarkConnected(ctx context.Context, callID, userID string) (*Call, error) {
	c, err := s.store.Update(ctx, callID, func(c *Call) error {
		if c.IsTerminal() {
			return domain.InvalidRequest("call already ended")
		}
		p := c.FindParticipant(userID)
		if p == nil {
			return domain.Forbidden("not a participant")
		}

		now := time.Now().UTC()
		p.State = ParticipantConnected
		if c.StartedAt == nil {
			c.StartedAt = &now
			c.State = StateConnected
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	s.publish(events.CallConnected, c, userID, nil)
	return c, nil
}

func (s *Service) Leave(ctx context.Context, callID, userID string) (*Call, error) {
	return s.endForUser(ctx, callID, userID, "left", false)
}

func (s *Service) Hangup(ctx context.Context, callID, userID string) (*Call, error) {
	return s.endForUser(ctx, callID, userID, "hangup", true)
}

func (s *Service) Fail(ctx context.Context, callID, reason string) (*Call, error) {
	c, err := s.store.Update(ctx, callID, func(c *Call) error {
		if c.IsTerminal() {
			return nil
		}
		now := time.Now().UTC()
		c.State = StateFailed
		c.EndedAt = &now
		c.Reason = reason
		return nil
	})

	if err != nil {
		return nil, err
	}
	s.publish(events.CallFailed, c, "", map[string]any{"reason": reason})
	return c, nil
}

func (s *Service) Resume(ctx context.Context, callID, userID, deviceID string) (*Call, error) {
	c, err := s.Get(ctx, callID)
	if err != nil {
		return nil, err
	}
	p := c.FindParticipant(userID)
	if p == nil {
		return nil, domain.Forbidden("not a participant")
	}

	if c.IsTerminal() {
		return c, nil
	}

	_, _ = s.store.Update(ctx, callID, func(c *Call) error {
		p := c.FindParticipant(userID)
		if p != nil {
			p.DeviceID = deviceID
			if p.State == ParticipantDisconnected || p.State == ParticipantReconnecting || p.State == ParticipantConnected {
				p.State = ParticipantReconnecting
			}
		}
		return nil
	})
	return s.Get(ctx, callID)
}

func (s *Service) ListActiveForUser(ctx context.Context, appID, userID string) []*Call {
	ids := s.store.ActiveCallIDsForUser(ctx, appID, userID)
	out := make([]*Call, 0, len(ids))
	for _, id := range ids {
		if c, err := s.Get(ctx, id); err == nil && c != nil && !c.IsTerminal() {
			out = append(out, c)
		}
	}
	return out
}

func (s *Service) endForUser(ctx context.Context, callID, userID, reason string, endCall bool) (*Call, error) {
	c, err := s.store.Update(ctx, callID, func(c *Call) error {
		if c.IsTerminal() {
			return domain.InvalidRequest("call already ended")
		}
		p := c.FindParticipant(userID)
		if p == nil {
			return domain.Forbidden("not a participant")
		}

		now := time.Now().UTC()
		p.State = ParticipantLeft
		p.LeftAt = &now

		if endCall || c.Mode == ModeDirect {
			c.State = StateEnded
			c.EndedAt = &now
			c.Reason = reason
			return nil
		}

		active := 0
		for _, other := range c.Participants {
			switch other.State {
			case ParticipantConnected, ParticipantConnecting, ParticipantAccepted, ParticipantReconnecting:
				active++
			}
		}
		if active == 0 {
			c.State = StateEnded
			c.EndedAt = &now
			c.Reason = reason
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	s.publish(events.CallParticipantLeft, c, userID, map[string]any{"reason": reason})
	if c.IsTerminal() {
		s.publish(events.CallEnded, c, userID, map[string]any{"reason": reason})
	}
	return c, nil
}

func (s *Service) publish(typ string, c *Call, sender string, extra map[string]any) {
	if s.bus == nil || c == nil {
		return
	}
	payload := map[string]any{
		"call": c,
	}
	for k, v := range extra {
		payload[k] = v
	}
	s.bus.Publish(events.Event{
		Type:    typ,
		AppID:   c.AppID,
		UserID:  sender,
		CallID:  c.ID,
		Payload: payload,
	})
}
