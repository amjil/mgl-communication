package provider

import (
	"context"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
)

// Named wraps an existing Provider under a different registry name
// (e.g. apns_voip sharing the APNs HTTP client).
type Named struct {
	name string
	inner Provider
}

func NewNamed(name string, inner Provider) *Named {
	return &Named{name: name, inner: inner}
}

func (n *Named) Name() string { return n.name }

func (n *Named) Send(ctx context.Context, message *domain.Message, device *domain.Device) (*SendResult, error) {
	return n.inner.Send(ctx, message, device)
}

func (n *Named) ValidateToken(ctx context.Context, token string) error {
	return n.inner.ValidateToken(ctx, token)
}

func (n *Named) Close() error { return nil }
