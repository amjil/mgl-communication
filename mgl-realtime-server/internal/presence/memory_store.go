package presence

import (
	"context"
	"sync"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/events"
)

// MemoryStore is an in-process Store used for unit tests and local runs without Redis.
type MemoryStore struct {
	mu      sync.RWMutex
	devices map[string]*DevicePresence     // device_id -> presence
	byUser  map[string]map[string]struct{} // appID|userID -> device set
	bus     *events.Bus
}

func NewMemoryStore(bus *events.Bus) *MemoryStore {
	return &MemoryStore{
		devices: make(map[string]*DevicePresence),
		byUser:  make(map[string]map[string]struct{}),
		bus:     bus,
	}
}

// NewStore returns a MemoryStore (kept for test convenience).
func NewStore(bus *events.Bus) *MemoryStore {
	return NewMemoryStore(bus)
}

func userKey(appID, userID string) string {
	return appID + "|" + userID
}

// SetOnline marks a device online and returns aggregated user presence.
func (s *MemoryStore) SetOnline(ctx context.Context, appID, userID, deviceID string) UserPresence {
	if ctx.Err() != nil {
		return UserPresence{UserID: userID, AppID: appID, Status: StatusOffline, UpdatedAt: time.Now().UTC()}
	}
	return s.update(appID, userID, deviceID, StatusOnline, events.PresenceOnline)
}

func (s *MemoryStore) SetOffline(ctx context.Context, appID, userID, deviceID string) UserPresence {
	if ctx.Err() != nil {
		return UserPresence{UserID: userID, AppID: appID, Status: StatusOffline, UpdatedAt: time.Now().UTC()}
	}
	return s.update(appID, userID, deviceID, StatusOffline, events.PresenceOffline)
}

func (s *MemoryStore) SetStatus(ctx context.Context, appID, userID, deviceID, status string) UserPresence {
	if ctx.Err() != nil {
		return UserPresence{UserID: userID, AppID: appID, Status: StatusOffline, UpdatedAt: time.Now().UTC()}
	}
	return s.update(appID, userID, deviceID, status, events.PresenceUpdated)
}

func (s *MemoryStore) update(appID, userID, deviceID, status, eventType string) UserPresence {
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

func (s *MemoryStore) GetUser(ctx context.Context, appID, userID string) UserPresence {
	if ctx.Err() != nil {
		return UserPresence{UserID: userID, AppID: appID, Status: StatusOffline, UpdatedAt: time.Now().UTC()}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotLocked(appID, userID)
}

func (s *MemoryStore) snapshotLocked(appID, userID string) UserPresence {
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
