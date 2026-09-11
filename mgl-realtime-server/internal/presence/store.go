package presence

import (
	"sync"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/events"
)

const (
	StatusOffline = "offline"
	StatusOnline  = "online"
	StatusIdle    = "idle"
	StatusBusy    = "busy"
	StatusInCall  = "in_call"
)

type DevicePresence struct {
	DeviceID  string    `json:"device_id"`
	UserID    string    `json:"user_id"`
	AppID     string    `json:"app_id"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserPresence struct {
	UserID    string           `json:"user_id"`
	AppID     string           `json:"app_id"`
	Status    string           `json:"status"`
	Devices   []DevicePresence `json:"devices"`
	UpdatedAt time.Time        `json:"updated_at"`
}

type Store struct {
	mu      sync.RWMutex
	devices map[string]*DevicePresence // device_id -> presence
	byUser  map[string]map[string]struct{} // appID|userID -> device set
	bus     *events.Bus
}

func NewStore(bus *events.Bus) *Store {
	return &Store{
		devices: make(map[string]*DevicePresence),
		byUser:  make(map[string]map[string]struct{}),
		bus:     bus,
	}
}

func userKey(appID, userID string) string {
	return appID + "|" + userID
}

// SetOnline marks a device online and returns aggregated user presence.
func (s *Store) SetOnline(appID, userID, deviceID string) UserPresence {
	return s.update(appID, userID, deviceID, StatusOnline, events.PresenceOnline)
}

func (s *Store) SetOffline(appID, userID, deviceID string) UserPresence {
	return s.update(appID, userID, deviceID, StatusOffline, events.PresenceOffline)
}

func (s *Store) SetStatus(appID, userID, deviceID, status string) UserPresence {
	return s.update(appID, userID, deviceID, status, events.PresenceUpdated)
}

func (s *Store) update(appID, userID, deviceID, status, eventType string) UserPresence {
	now := time.Now().UTC()
	s.mu.Lock()
	s.devices[deviceID] = &DevicePresence{
		DeviceID:  deviceID,
		UserID:    userID,
		AppID:     appID,
		Status:    status,
		UpdatedAt: now,
	}
	uk := userKey(appID, userID)
	if s.byUser[uk] == nil {
		s.byUser[uk] = make(map[string]struct{})
	}
	s.byUser[uk][deviceID] = struct{}{}
	up := s.snapshotLocked(appID, userID)
	s.mu.Unlock()

	if s.bus != nil {
		s.bus.Publish(events.Event{
			Type:     eventType,
			AppID:    appID,
			UserID:   userID,
			DeviceID: deviceID,
			Payload: map[string]any{
				"status": up.Status,
			},
		})
	}
	return up
}

func (s *Store) GetUser(appID, userID string) UserPresence {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotLocked(appID, userID)
}

func (s *Store) snapshotLocked(appID, userID string) UserPresence {
	uk := userKey(appID, userID)
	ids := s.byUser[uk]
	devices := make([]DevicePresence, 0, len(ids))
	agg := StatusOffline
	var updated time.Time
	for id := range ids {
		d := s.devices[id]
		if d == nil {
			continue
		}
		devices = append(devices, *d)
		if d.UpdatedAt.After(updated) {
			updated = d.UpdatedAt
		}
		agg = mergeStatus(agg, d.Status)
	}
	if updated.IsZero() {
		updated = time.Now().UTC()
	}
	return UserPresence{
		UserID:    userID,
		AppID:     appID,
		Status:    agg,
		Devices:   devices,
		UpdatedAt: updated,
	}
}

func mergeStatus(a, b string) string {
	rank := map[string]int{
		StatusOffline: 0,
		StatusIdle:    1,
		StatusOnline:  2,
		StatusBusy:    3,
		StatusInCall:  4,
	}
	if rank[b] > rank[a] {
		return b
	}
	return a
}
