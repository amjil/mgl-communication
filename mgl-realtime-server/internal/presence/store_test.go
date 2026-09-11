package presence_test

import (
	"testing"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/presence"
)

func TestPresenceAggregation(t *testing.T) {
	store := presence.NewStore(nil)
	store.SetOnline("app", "user", "d1")
	up := store.SetStatus("app", "user", "d2", presence.StatusInCall)
	if up.Status != presence.StatusInCall {
		t.Fatalf("status=%s", up.Status)
	}
	if len(up.Devices) != 2 {
		t.Fatalf("devices=%d", len(up.Devices))
	}
	up = store.SetOffline("app", "user", "d2")
	if up.Status != presence.StatusOnline {
		t.Fatalf("status after offline=%s", up.Status)
	}
}
