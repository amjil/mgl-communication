package fcm

import (
	"testing"
	"time"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
)

func TestBuildMessageNotificationAndData(t *testing.T) {
	ttl := 60 * time.Second
	msg := &domain.Message{
		ID:          "01KTEST",
		Title:       "Nomio",
		Body:        "hello",
		Data:        map[string]string{"type": "comment", "article_id": "123"},
		DeepLink:    "nomio://article/123",
		Priority:    domain.PriorityHigh,
		TTL:         ttl,
		CollapseKey: "c1",
		Sound:       "default",
		Category:    "mgl_default",
		ImageURL:    "https://example.com/a.png",
	}
	device := &domain.Device{Token: "tok123"}

	out := buildMessage(msg, device)
	if out.Token != "tok123" {
		t.Fatalf("token=%s", out.Token)
	}
	if out.Data["mgl_message_id"] != "01KTEST" {
		t.Fatalf("missing mgl_message_id: %#v", out.Data)
	}
	if out.Data["deep_link"] != "nomio://article/123" {
		t.Fatal("deep_link missing")
	}
	if out.Data["type"] != "comment" {
		t.Fatal("data type")
	}
	if out.Notification == nil || out.Notification.Title != "Nomio" {
		t.Fatal("notification")
	}
	if out.Android == nil || out.Android.Priority != "high" {
		t.Fatal("android priority")
	}
	if out.Android.CollapseKey != "c1" {
		t.Fatal("collapse")
	}
	if out.Android.Notification == nil || out.Android.Notification.ChannelID != "mgl_default" {
		t.Fatal("channel")
	}
}

func TestBuildMessageDataOnly(t *testing.T) {
	out := buildMessage(&domain.Message{
		ID:   "id1",
		Data: map[string]string{"type": "sync"},
	}, &domain.Device{Token: "t"})
	if out.Notification != nil {
		t.Fatal("expected no notification")
	}
	if out.Data["type"] != "sync" {
		t.Fatal("data")
	}
}

func TestMapSendErrorInvalidToken(t *testing.T) {
	res, err := mapSendError(fmtError("Requested entity was not found. registration-token-not-registered"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.InvalidToken || res.Accepted {
		t.Fatalf("%+v", res)
	}
}

func TestMapSendErrorRateLimit(t *testing.T) {
	res, err := mapSendError(fmtError("quota exceeded 429"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Retryable || res.ErrorCode != domain.ErrCodeProviderRateLimit {
		t.Fatalf("%+v", res)
	}
}

type simpleErr string

func (e simpleErr) Error() string { return string(e) }

func fmtError(s string) error { return simpleErr(s) }
