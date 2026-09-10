package domain

import "time"

const (
	DeliveryStatusPending  = "pending"
	DeliveryStatusSending  = "sending"
	DeliveryStatusAccepted = "accepted"
	DeliveryStatusDelivered = "delivered"
	DeliveryStatusOpened   = "opened"
	DeliveryStatusFailed   = "failed"
	DeliveryStatusRetrying = "retrying"
)

type Delivery struct {
	ID                string
	MessageID         string
	DeviceID          string
	Provider          string
	Status            string
	ProviderMessageID string
	ErrorCode         string
	ErrorMessage      string
	Attempts          int
	CreatedAt         time.Time
	SentAt            *time.Time
	DeliveredAt       *time.Time
	OpenedAt          *time.Time
	NextRetryAt       *time.Time
}
