package service_test

import (
	"testing"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
	"github.com/amjil/mgl-push/mgl-push-server/internal/service"
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
