package call

import (
	"context"
	"sync"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
)

// MemoryStore is an in-process Store used for unit tests and local runs without Redis.
type MemoryStore struct {
	mu     sync.RWMutex
	calls  map[string]*Call
	byUser map[string]map[string]struct{} // app|user -> callIDs
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		calls:  make(map[string]*Call),
		byUser: make(map[string]map[string]struct{}),
	}
}

// NewStore returns a MemoryStore (kept for test convenience).
func NewStore() *MemoryStore {
	return NewMemoryStore()
}

func (s *MemoryStore) Put(ctx context.Context, c *Call) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls[c.ID] = c
	for _, p := range c.Participants {
		uk := c.AppID + "|" + p.UserID
		if s.byUser[uk] == nil {
			s.byUser[uk] = make(map[string]struct{})
		}
		s.byUser[uk][c.ID] = struct{}{}
	}
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, callID string) (*Call, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.calls[callID]
	if !ok {
		return nil, domain.CallNotFound()
	}
	return c.Clone(), nil
}

func (s *MemoryStore) Update(ctx context.Context, callID string, fn func(*Call) error) (*Call, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.calls[callID]
	if !ok {
		return nil, domain.CallNotFound()
	}
	if err := fn(c); err != nil {
		return nil, err
	}
	// Keep user index in sync when Join adds participants.
	for _, p := range c.Participants {
		uk := c.AppID + "|" + p.UserID
		if s.byUser[uk] == nil {
			s.byUser[uk] = make(map[string]struct{})
		}
		s.byUser[uk][c.ID] = struct{}{}
	}
	return c.Clone(), nil
}

func (s *MemoryStore) ActiveCallIDsForUser(ctx context.Context, appID, userID string) []string {
	if ctx.Err() != nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byUser[appID+"|"+userID]
	out := make([]string, 0, len(ids))
	for id := range ids {
		if c, ok := s.calls[id]; ok && !c.IsTerminal() {
			out = append(out, id)
		}
	}
	return out
}
