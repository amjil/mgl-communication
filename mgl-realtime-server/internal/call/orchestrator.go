package call

import (
	"context"
	"fmt"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/auth"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/config"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/phoenix"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/sfu"
)

// Orchestrator adds Phoenix authorization, SFU tokens, and ICE config on top of runtime.
type Orchestrator struct {
	Runtime   *Service
	Authz     phoenix.Authorizer
	SFU       *sfu.Service
	JWT       *auth.JWTValidator
	TokenTTL  time.Duration
	ICEServers []config.ICEServer
}

type TokenResult struct {
	CallID     string             `json:"call_id"`
	RoomID     string             `json:"room_id"`
	Token      string             `json:"token"`
	Transport  string             `json:"transport"`
	SFUURL     string             `json:"sfu_url,omitempty"`
	ICEServers []config.ICEServer `json:"ice_servers,omitempty"`
	Role       string             `json:"role"`
	ExpiresIn  int                `json:"expires_in"`
}

// ResumeResult is the call recovery payload (spec §26).
type ResumeResult struct {
	CallID       string             `json:"call_id"`
	State        string             `json:"state"`
	Participants []*Participant     `json:"participants"`
	Room         map[string]any     `json:"room"`
	Call         *Call              `json:"call"`
	Token        *TokenResult       `json:"token,omitempty"`
	ICEServers   []config.ICEServer `json:"ice_servers,omitempty"`
}

func (o *Orchestrator) Create(in CreateInput) (*Call, error) {
	return o.CreateCtx(context.Background(), in)
}

func (o *Orchestrator) CreateCtx(ctx context.Context, in CreateInput) (*Call, error) {
	if o.Authz != nil {
		if err := o.Authz.CanUserCall(ctx, in.AppID, in.CallerID, in.CalleeIDs); err != nil {
			return nil, domain.Forbidden(fmt.Sprintf("not allowed: %v", err))
		}
	}
	c, err := o.Runtime.Create(in)
	if err != nil {
		return nil, err
	}
	if c.Transport == TransportSFU && o.SFU != nil {
		if err := o.SFU.EnsureRoom(ctx, c.RoomID); err != nil {
			_, _ = o.Runtime.Fail(c.ID, "sfu_room_failed")
			return nil, domain.Internal("sfu room create failed")
		}
	}
	return c, nil
}

func (o *Orchestrator) IssueToken(ctx context.Context, callID, userID string) (*TokenResult, error) {
	c, err := o.Runtime.Get(callID)
	if err != nil {
		return nil, err
	}
	p := c.FindParticipant(userID)
	if p == nil {
		return nil, domain.Forbidden("not a participant")
	}
	role := "publisher"
	if p.Role != "" {
		role = p.Role
	}
	ttl := o.TokenTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	result := &TokenResult{
		CallID:    c.ID,
		RoomID:    c.RoomID,
		Transport: c.Transport,
		Role:      role,
		ExpiresIn: int(ttl.Seconds()),
	}
	if c.Transport == TransportSFU {
		info, err := o.SFU.Join(ctx, c.RoomID, userID, role, ttl)
		if err != nil {
			return nil, domain.Internal("sfu token failed")
		}
		result.Token = info.Token
		result.SFUURL = info.URL
	} else {
		tok, err := o.JWT.IssueCallToken(c.ID, userID, c.RoomID, role, c.AppID, ttl)
		if err != nil {
			return nil, domain.Internal("call token failed")
		}
		result.Token = tok
		result.ICEServers = o.ICEServers
	}
	return result, nil
}

// Delegate runtime methods used by Hub / HTTP.
func (o *Orchestrator) Accept(callID, userID, deviceID string) (*Call, error) {
	return o.Runtime.Accept(callID, userID, deviceID)
}
func (o *Orchestrator) Reject(callID, userID string) (*Call, error) {
	return o.Runtime.Reject(callID, userID)
}
func (o *Orchestrator) Cancel(callID, userID string) (*Call, error) {
	return o.Runtime.Cancel(callID, userID)
}
func (o *Orchestrator) Join(callID, userID, deviceID string) (*Call, error) {
	return o.Runtime.Join(callID, userID, deviceID)
}
func (o *Orchestrator) Leave(callID, userID string) (*Call, error) {
	return o.Runtime.Leave(callID, userID)
}
func (o *Orchestrator) Hangup(callID, userID string) (*Call, error) {
	return o.Runtime.Hangup(callID, userID)
}
func (o *Orchestrator) Resume(callID, userID, deviceID string) (*Call, error) {
	return o.Runtime.Resume(callID, userID, deviceID)
}

func (o *Orchestrator) ResumeFull(ctx context.Context, callID, userID, deviceID string) (*ResumeResult, error) {
	c, err := o.Runtime.Resume(callID, userID, deviceID)
	if err != nil {
		return nil, err
	}
	result := &ResumeResult{
		CallID:       c.ID,
		State:        c.State,
		Participants: c.Participants,
		Call:         c,
		Room: map[string]any{
			"room_id":   c.RoomID,
			"transport": c.Transport,
		},
		ICEServers: o.ICEServers,
	}
	if !c.IsTerminal() {
		tok, err := o.IssueToken(ctx, callID, userID)
		if err == nil {
			result.Token = tok
			if tok.SFUURL != "" {
				result.Room["sfu_url"] = tok.SFUURL
			}
			if len(tok.ICEServers) > 0 {
				result.ICEServers = tok.ICEServers
			}
		}
	}
	return result, nil
}

func (o *Orchestrator) ListActiveForUser(appID, userID string) []*Call {
	return o.Runtime.ListActiveForUser(appID, userID)
}

func (o *Orchestrator) MarkConnected(callID, userID string) (*Call, error) {
	return o.Runtime.MarkConnected(callID, userID)
}
func (o *Orchestrator) Get(callID string) (*Call, error) {
	return o.Runtime.Get(callID)
}
