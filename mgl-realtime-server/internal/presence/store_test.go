package presence_test

import (
	"context"
	"testing"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/presence"
)

func TestPresenceAggregation(t *testing.T) {
	ctx := context.Background()
	store := presence.NewMemoryStore(nil)
	store.SetOnline(ctx, "app", "user", "d1")
	up := store.SetStatus(ctx, "app", "user", "d2", presence.StatusInCall)
	if up.Status != presence.StatusInCall {
		t.Fatalf("status=%s", up.Status)
	}
	if len(up.Devices) != 2 {
		t.Fatalf("devices=%d", len(up.Devices))
	}
	up = store.SetOffline(ctx, "app", "user", "d2")
	if up.Status != presence.StatusOnline {
		t.Fatalf("status after offline=%s", up.Status)
	}
}
