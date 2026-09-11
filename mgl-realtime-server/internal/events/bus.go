package events

import (
	"sync"
	"time"
)

// Event is an internal realtime event (spec §27).
// JSON tags are required for Redis Pub/Sub serialization.
type Event struct {
	Type      string         `json:"type"`
	AppID     string         `json:"app_id"`
	UserID    string         `json:"user_id"`
	DeviceID  string         `json:"device_id,omitempty"`
	CallID    string         `json:"call_id,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type Handler func(Event)

// Bus is a process-local pub/sub (Go channels; Phase 1).
type Bus struct {
	mu       sync.RWMutex
	subs     map[string][]Handler
	all      []Handler
	buffer   int
}

func NewBus(buffer int) *Bus {
	if buffer <= 0 {
		buffer = 256
	}
	return &Bus{
		subs:   make(map[string][]Handler),
		buffer: buffer,
	}
}

func (b *Bus) Subscribe(eventType string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[eventType] = append(b.subs[eventType], h)
}

func (b *Bus) SubscribeAll(h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.all = append(b.all, h)
}

func (b *Bus) Publish(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	b.mu.RLock()
	handlers := append([]Handler{}, b.subs[e.Type]...)
	handlers = append(handlers, b.all...)
	b.mu.RUnlock()
	for _, h := range handlers {
		h := h
		go func() {
			defer func() { _ = recover() }()
			h(e)
		}()
	}
}

// Common event type constants.
const (
	CallCreated           = "call.created"
	CallRinging           = "call.ringing"
	CallAccepted          = "call.accepted"
	CallRejected          = "call.rejected"
	CallCancelled         = "call.cancelled"
	CallConnected         = "call.connected"
	CallParticipantJoined = "call.participant.joined"
	CallParticipantLeft   = "call.participant.left"
	CallEnded             = "call.ended"
	CallFailed            = "call.failed"

	DeviceConnected    = "device.connected"
	DeviceDisconnected = "device.disconnected"

	PresenceOnline  = "presence.online"
	PresenceOffline = "presence.offline"
	PresenceUpdated = "presence.updated"

	PushRequested = "push.requested"
	PushSent      = "push.sent"
	PushFailed    = "push.failed"

	// WSDeliver is published on user:{app_id}:{user_id} topics for cross-node WS fanout.
	// Payload["envelope"] holds the protocol.Envelope JSON; DeviceID is except-device filter.
	WSDeliver = "ws.deliver"
)
