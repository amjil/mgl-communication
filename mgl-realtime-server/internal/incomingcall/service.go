package incomingcall

import (
	"context"
	"log/slog"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/call"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/events"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/service"
)

// Service bridges Call Runtime → Push (spec §12 / §24).
type Service struct {
	messages *service.MessageService
	bus      *events.Bus
	logger   *slog.Logger
}

func New(messages *service.MessageService, bus *events.Bus, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{messages: messages, bus: bus, logger: logger}
}

func (s *Service) NotifyRinging(ctx context.Context, c *call.Call, callerName string) {
	if s.messages == nil || c == nil {
		return
	}
	callees := make([]string, 0)
	for _, p := range c.Participants {
		if p.UserID == c.CallerID {
			continue
		}
		if p.State == call.ParticipantRinging || p.State == call.ParticipantInvited {
			callees = append(callees, p.UserID)
		}
	}
	if len(callees) == 0 {
		return
	}
	media := c.Type
	expires := time.Now().UTC().Add(domain.DefaultIncomingCallTTL)
	for _, callee := range callees {
		_, err := s.messages.SendIncomingCall(ctx, service.SendIncomingCallInput{
			AppID:   c.AppID,
			UserIDs: []string{callee},
			Call: domain.IncomingCall{
				CallID:     c.ID,
				CallerID:   c.CallerID,
				CalleeID:   callee,
				MediaType:  media,
				CallerName: callerName,
				Timestamp:  time.Now().UTC(),
				ExpiresAt:  expires,
			},
			IdempotencyKey: "incoming:" + c.ID + ":" + callee,
		})
		if err != nil {
			s.logger.Error("incoming call push failed", "call_id", c.ID, "user_id", callee, "error", err)
			if s.bus != nil {
				s.bus.Publish(events.Event{Type: events.PushFailed, AppID: c.AppID, CallID: c.ID, UserID: callee})
			}
			continue
		}
		if s.bus != nil {
			s.bus.Publish(events.Event{Type: events.PushRequested, AppID: c.AppID, CallID: c.ID, UserID: callee})
		}
	}
}

func (s *Service) NotifyCancelled(ctx context.Context, c *call.Call, reason string) {
	if s.messages == nil || c == nil {
		return
	}
	userIDs := make([]string, 0)
	for _, p := range c.Participants {
		if p.UserID != c.CallerID {
			userIDs = append(userIDs, p.UserID)
		}
	}
	s.notifyUsers(ctx, c, userIDs, reason, "cancel:"+c.ID)
}

// NotifyStopRinging sends call_cancelled to a user's devices so other handsets
// stop ringing after one device accepted (spec §24).
func (s *Service) NotifyStopRinging(ctx context.Context, c *call.Call, userID, reason string) {
	if s.messages == nil || c == nil || userID == "" {
		return
	}
	key := "stop_ring:" + c.ID + ":" + userID
	s.notifyUsers(ctx, c, []string{userID}, reason, key)
}

func (s *Service) notifyUsers(ctx context.Context, c *call.Call, userIDs []string, reason, idempotencyKey string) {
	if len(userIDs) == 0 {
		return
	}
	if reason == "" {
		reason = "cancelled"
	}
	_, err := s.messages.SendCallSignal(ctx, service.SendCallSignalInput{
		AppID:          c.AppID,
		Type:           domain.MessageCallCancelled,
		CallID:         c.ID,
		UserIDs:        userIDs,
		Reason:         reason,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		s.logger.Error("call cancelled push failed", "call_id", c.ID, "error", err)
		if s.bus != nil {
			s.bus.Publish(events.Event{Type: events.PushFailed, AppID: c.AppID, CallID: c.ID})
		}
		return
	}
	if s.bus != nil {
		s.bus.Publish(events.Event{Type: events.PushSent, AppID: c.AppID, CallID: c.ID})
	}
}
