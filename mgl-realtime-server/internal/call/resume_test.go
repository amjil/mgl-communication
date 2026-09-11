package call_test

import (
	"context"
	"testing"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/auth"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/call"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/config"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/phoenix"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/sfu"
)

func TestResumeFullIncludesRoomAndToken(t *testing.T) {
	ctx := context.Background()
	jwt := auth.NewJWTValidator("secret", "test")
	orch := &call.Orchestrator{
		Runtime:    call.NewService(call.NewStore(), nil, time.Minute),
		Authz:      phoenix.AllowAll{},
		SFU:        sfu.NewService(sfu.NewStub(""), []config.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}}),
		JWT:        jwt,
		TokenTTL:   time.Minute,
		ICEServers: []config.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	}
	c, err := orch.Create(ctx, call.CreateInput{
		AppID: "nomio", CallerID: "a", CalleeIDs: []string{"b"}, Type: call.TypeAudio,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = orch.Accept(ctx, c.ID, "b", "d1")
	snap, err := orch.ResumeFull(ctx, c.ID, "b", "d1")
	if err != nil {
		t.Fatal(err)
	}
	if snap.CallID != c.ID || snap.Room["room_id"] == nil {
		t.Fatalf("snapshot=%+v", snap)
	}
	if snap.Token == nil || snap.Token.Token == "" {
		t.Fatal("expected call token")
	}
	if len(snap.ICEServers) == 0 {
		t.Fatal("expected ice servers")
	}
}
