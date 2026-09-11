package service_test

import (
	"testing"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/service"
)

func TestSendValidation(t *testing.T) {
	svc := service.NewMessageService(nil, nil, nil)
	_, err := svc.Send(nil, service.SendMessageInput{AppID: "app"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.(*domain.AppError).Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("got %v", err)
	}
}

func TestIncomingCallValidation(t *testing.T) {
	svc := service.NewMessageService(nil, nil, nil)
	_, err := svc.SendIncomingCall(nil, service.SendIncomingCallInput{
		AppID:   "app",
		UserIDs: []string{"u1"},
		Call:    domain.IncomingCall{CallID: "c1"},
	})
	if err == nil {
		t.Fatal("expected error for missing caller/callee")
	}
}

func TestIncomingCallExpired(t *testing.T) {
	svc := service.NewMessageService(nil, nil, nil)
	_, err := svc.SendIncomingCall(nil, service.SendIncomingCallInput{
		AppID:   "app",
		UserIDs: []string{"u1"},
		Call: domain.IncomingCall{
			CallID:    "c1",
			CallerID:  "a",
			CalleeID:  "b",
			ExpiresAt: time.Now().Add(-time.Second),
		},
	})
	if err == nil {
		t.Fatal("expected expired error")
	}
}

func TestMessageTypeIsDataOnly(t *testing.T) {
	if !domain.MessageIncomingCall.IsDataOnly() {
		t.Fatal("incoming_call should be data-only")
	}
	if domain.MessageNotification.IsDataOnly() {
		t.Fatal("notification should not be data-only")
	}
}
