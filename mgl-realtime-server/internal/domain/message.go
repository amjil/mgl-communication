package domain

import "time"

type MessageType string

const (
	MessageNotification  MessageType = "notification"
	MessageSilent        MessageType = "silent"
	MessageBackground    MessageType = "background"
	MessageIncomingCall  MessageType = "incoming_call"
	MessageCallCancelled MessageType = "call_cancelled"
	MessageCallEnded     MessageType = "call_ended"
)

const (
	PriorityNormal = "normal"
	PriorityHigh   = "high"

	MessageStatusCreated   = "created"
	MessageStatusQueued    = "queued"
	MessageStatusSending   = "sending"
	MessageStatusAccepted  = "accepted"
	MessageStatusCompleted = "completed"
	MessageStatusFailed    = "failed"

	DefaultIncomingCallTTL = 30 * time.Second
)

type Message struct {
	ID          string
	Type        MessageType
	Title       string
	Body        string
	Data        map[string]string
	ImageURL    string
	Priority    string
	TTL         time.Duration
	CollapseKey string
	Sound       string
	Badge       *int
	DeepLink    string
	Category    string
	AppID       string
	Status      string
	CreatedAt   time.Time
	QueuedAt    *time.Time
	CompletedAt *time.Time
}

// IncomingCall is the dedicated call-push payload (spec §44).
type IncomingCall struct {
	CallID     string
	CallerID   string
	CalleeID   string
	MediaType  string
	CallerName string
	Timestamp  time.Time
	ExpiresAt  time.Time
}

type SendTarget struct {
	UserIDs         []string
	InstallationIDs []string
	Provider        string
	Token           string
}

func (t MessageType) IsCallRelated() bool {
	switch t {
	case MessageIncomingCall, MessageCallCancelled, MessageCallEnded:
		return true
	default:
		return false
	}
}

func (t MessageType) IsDataOnly() bool {
	switch t {
	case MessageSilent, MessageBackground, MessageIncomingCall, MessageCallCancelled, MessageCallEnded:
		return true
	default:
		return false
	}
}
