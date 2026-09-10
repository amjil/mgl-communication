package provider

import (
	"context"
	"fmt"
	"sync"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
)

type SendResult struct {
	Accepted          bool
	ProviderMessageID string
	Retryable         bool
	ErrorCode         string
	ErrorMessage      string
	InvalidToken      bool
}

type ProviderError struct {
	Provider     string
	Code         string
	Message      string
	Retryable    bool
	InvalidToken bool
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.Provider, e.Code, e.Message)
}

type Provider interface {
	Name() string
	Send(ctx context.Context, message *domain.Message, device *domain.Device) (*SendResult, error)
	ValidateToken(ctx context.Context, token string) error
	Close() error
}

type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	if !ok {
		return nil, &ProviderError{
			Provider: name,
			Code:     domain.ErrCodeProviderNotAvailable,
			Message:  fmt.Sprintf("provider %q not registered", name),
		}
	}
	return p, nil
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for n := range r.providers {
		names = append(names, n)
	}
	return names
}

func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var firstErr error
	for _, p := range r.providers {
		if err := p.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
