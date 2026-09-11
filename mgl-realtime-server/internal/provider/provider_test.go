package provider_test

import (
	"context"
	"testing"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/provider"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/provider/noop"
)

func TestRegistryAndNoopSend(t *testing.T) {
	reg := provider.NewRegistry()
	reg.Register(noop.New(domain.ProviderFCM))

	p, err := reg.Get(domain.ProviderFCM)
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Send(context.Background(), &domain.Message{ID: "m1", Title: "t"}, &domain.Device{Token: "tok"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Accepted {
		t.Fatal("expected accepted")
	}

	res, err = p.Send(context.Background(), &domain.Message{ID: "m2"}, &domain.Device{Token: "invalid"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.InvalidToken {
		t.Fatal("expected invalid token")
	}
}
