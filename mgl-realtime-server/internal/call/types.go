package call

import (
	"time"
)

const (
	StateCreated    = "created"
	StateRinging    = "ringing"
	StateAccepted   = "accepted"
	StateConnecting = "connecting"
	StateConnected  = "connected"
	StateEnded      = "ended"
	StateRejected   = "rejected"
	StateCancelled  = "cancelled"
	StateTimeout    = "timeout"
	StateFailed     = "failed"
)

const (
	ParticipantInvited       = "invited"
	ParticipantRinging       = "ringing"
	ParticipantAccepted      = "accepted"
	ParticipantConnecting    = "connecting"
	ParticipantConnected     = "connected"
	ParticipantLeft          = "left"
	ParticipantReconnecting  = "reconnecting"
	ParticipantDisconnected  = "disconnected"
	ParticipantRejected      = "rejected"
)

const (
	TypeAudio = "audio"
	TypeVideo = "video"

	ModeDirect = "direct"
	ModeGroup  = "group"

	TransportP2P = "p2p"
	TransportSFU = "sfu"
)

type Call struct {
	ID          string         `json:"call_id"`
	RoomID      string         `json:"room_id"`
	AppID       string         `json:"app_id"`
	CallerID    string         `json:"caller_id"`
	Type        string         `json:"type"`
	Mode        string         `json:"mode"`
	State       string         `json:"state"`
	Transport   string         `json:"transport"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	EndedAt     *time.Time     `json:"ended_at,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	Participants []*Participant `json:"participants"`
}

type Participant struct {
	CallID    string     `json:"call_id"`
	UserID    string     `json:"user_id"`
	DeviceID  string     `json:"device_id,omitempty"`
	State     string     `json:"state"`
	JoinedAt  *time.Time `json:"joined_at,omitempty"`
	LeftAt    *time.Time `json:"left_at,omitempty"`
	Role      string     `json:"role,omitempty"`
}

func (c *Call) IsTerminal() bool {
	switch c.State {
	case StateEnded, StateRejected, StateCancelled, StateTimeout, StateFailed:
		return true
	default:
		return false
	}
}

func (c *Call) FindParticipant(userID string) *Participant {
	for _, p := range c.Participants {
		if p.UserID == userID {
			return p
		}
	}
	return nil
}

func (c *Call) Clone() *Call {
	cp := *c
	cp.Participants = make([]*Participant, len(c.Participants))
	for i, p := range c.Participants {
		pp := *p
		cp.Participants[i] = &pp
	}
	if c.StartedAt != nil {
		t := *c.StartedAt
		cp.StartedAt = &t
	}
	if c.EndedAt != nil {
		t := *c.EndedAt
		cp.EndedAt = &t
	}
	return &cp
}
