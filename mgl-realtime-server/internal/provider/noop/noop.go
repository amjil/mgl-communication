package noop

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/provider"
)

// Provider is a Phase-1 stub that accepts all messages without calling a real vendor.
type Provider struct {
	name  string
	seq   atomic.Uint64
}

func New(name string) *Provider {
	return &Provider{name: name}
}

func (p *Provider) Name() string { return p.name }

func (p *Provider) Send(_ context.Context, message *domain.Message, device *domain.Device) (*provider.SendResult, error) {
	if device.Token == "" || device.Token == "invalid" {
		return &provider.SendResult{
			Accepted:     false,
			Retryable:    false,
			InvalidToken: true,
			ErrorCode:    domain.ErrCodeInvalidToken,
			ErrorMessage: "invalid token",
		}, nil
	}
	id := p.seq.Add(1)
	return &provider.SendResult{
		Accepted:          true,
		ProviderMessageID: fmt.Sprintf("%s-%s-%d", p.name, message.ID, id),
	}, nil
}

func (p *Provider) ValidateToken(_ context.Context, token string) error {
	if token == "" || token == "invalid" {
		return &provider.ProviderError{
			Provider:     p.name,
			Code:         domain.ErrCodeInvalidToken,
			Message:      "invalid token",
			InvalidToken: true,
		}
	}
	return nil
}

func (p *Provider) Close() error { return nil }
