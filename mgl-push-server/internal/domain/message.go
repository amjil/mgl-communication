package domain

import "time"

const (
	PriorityNormal = "normal"
	PriorityHigh   = "high"

	MessageStatusCreated   = "created"
	MessageStatusQueued    = "queued"
	MessageStatusSending   = "sending"
	MessageStatusCompleted = "completed"
	MessageStatusFailed    = "failed"
)

type Message struct {
	ID          string
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

type SendTarget struct {
	UserIDs          []string
	InstallationIDs  []string
	Provider         string
	Token            string
}
