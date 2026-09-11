package service_test

import (
	"testing"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/service"
)

func TestValidateRegisterViaRegisterErrors(t *testing.T) {
	svc := service.NewDeviceService(nil)
	_, err := svc.Register(nil, service.RegisterDeviceInput{})
	if err == nil {
		t.Fatal("expected error")
	}
	appErr, ok := err.(*domain.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("code=%s", appErr.Code)
	}
}

func TestMaskToken(t *testing.T) {
	got := service.MaskToken("abcdefghijklmnopxyz")
	if got != "abcde...xyz" {
		t.Fatalf("got %s", got)
	}
}
