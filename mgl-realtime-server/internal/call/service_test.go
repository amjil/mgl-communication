package call_test

import (
	"context"
	"testing"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/call"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/events"
)

func TestDirectCallLifecycle(t *testing.T) {
	ctx := context.Background()
	bus := events.NewBus(16)
	svc := call.NewService(call.NewStore(), bus, time.Minute)

	c, err := svc.Create(ctx, call.CreateInput{
		AppID:     "app",
		CallerID:  "user_a",
		CalleeIDs: []string{"user_b"},
		Type:      call.TypeVideo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.State != call.StateRinging {
		t.Fatalf("state=%s", c.State)
	}
	if c.Transport != call.TransportP2P {
		t.Fatalf("transport=%s", c.Transport)
	}
	if c.Mode != call.ModeDirect {
		t.Fatalf("mode=%s", c.Mode)
	}

	c, err = svc.Accept(ctx, c.ID, "user_b", "device_b")
	if err != nil {
		t.Fatal(err)
	}
	if c.State != call.StateAccepted {
		t.Fatalf("state=%s", c.State)
	}

	c, err = svc.Join(ctx, c.ID, "user_b", "device_b")
	if err != nil {
		t.Fatal(err)
	}
	c, err = svc.MarkConnected(ctx, c.ID, "user_b")
	if err != nil {
		t.Fatal(err)
	}
	if c.State != call.StateConnected {
		t.Fatalf("state=%s", c.State)
	}

	c, err = svc.Hangup(ctx, c.ID, "user_a")
	if err != nil {
		t.Fatal(err)
	}
	if c.State != call.StateEnded {
		t.Fatalf("state=%s", c.State)
	}
}

func TestGroupCallUsesSFU(t *testing.T) {
	ctx := context.Background()
	svc := call.NewService(call.NewStore(), nil, time.Minute)
	c, err := svc.Create(ctx, call.CreateInput{
		AppID:     "app",
		CallerID:  "user_a",
		CalleeIDs: []string{"user_b", "user_c"},
		Type:      call.TypeAudio,
		Mode:      call.ModeGroup,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Transport != call.TransportSFU {
		t.Fatalf("transport=%s", c.Transport)
	}
}

func TestRejectEndsDirectCall(t *testing.T) {
	ctx := context.Background()
	svc := call.NewService(call.NewStore(), nil, time.Minute)
	c, err := svc.Create(ctx, call.CreateInput{
		AppID: "app", CallerID: "a", CalleeIDs: []string{"b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err = svc.Reject(ctx, c.ID, "b")
	if err != nil {
		t.Fatal(err)
	}
	if c.State != call.StateRejected {
		t.Fatalf("state=%s", c.State)
	}
}

func TestMultiDeviceAcceptMarksDevice(t *testing.T) {
	ctx := context.Background()
	svc := call.NewService(call.NewStore(), nil, time.Minute)
	c, err := svc.Create(ctx, call.CreateInput{
		AppID: "app", CallerID: "a", CalleeIDs: []string{"b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err = svc.Accept(ctx, c.ID, "b", "iphone")
	if err != nil {
		t.Fatal(err)
	}
	p := c.FindParticipant("b")
	if p == nil || p.DeviceID != "iphone" || p.State != call.ParticipantAccepted {
		t.Fatalf("participant=%+v", p)
	}
}

func TestResumeMarksReconnecting(t *testing.T) {
	ctx := context.Background()
	svc := call.NewService(call.NewStore(), nil, time.Minute)
	c, err := svc.Create(ctx, call.CreateInput{
		AppID: "app", CallerID: "a", CalleeIDs: []string{"b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = svc.Accept(ctx, c.ID, "b", "d1")
	_, _ = svc.MarkConnected(ctx, c.ID, "b")
	c, err = svc.Resume(ctx, c.ID, "b", "d1")
	if err != nil {
		t.Fatal(err)
	}
	p := c.FindParticipant("b")
	if p.State != call.ParticipantReconnecting {
		t.Fatalf("state=%s", p.State)
	}
}

func TestListActiveForUser(t *testing.T) {
	ctx := context.Background()
	svc := call.NewService(call.NewStore(), nil, time.Minute)
	c, err := svc.Create(ctx, call.CreateInput{
		AppID: "app", CallerID: "a", CalleeIDs: []string{"b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	active := svc.ListActiveForUser(ctx, "app", "b")
	if len(active) != 1 || active[0].ID != c.ID {
		t.Fatalf("active=%v", active)
	}
	_, _ = svc.Hangup(ctx, c.ID, "a")
	if len(svc.ListActiveForUser(ctx, "app", "b")) != 0 {
		t.Fatal("expected no active calls")
	}
}
