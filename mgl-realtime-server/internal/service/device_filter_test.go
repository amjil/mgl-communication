package service

import (
	"testing"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
)

func TestFilterDevicesForMessagePrefersVoIPForIncoming(t *testing.T) {
	devices := []*domain.Device{
		{ID: "1", AppID: "app", InstallationID: "inst1", Platform: domain.PlatformIOS, Provider: domain.ProviderAPNs},
		{ID: "2", AppID: "app", InstallationID: "inst1", Platform: domain.PlatformIOS, Provider: domain.ProviderAPNsVoIP},
	}
	out := filterDevicesForMessage(devices, domain.MessageIncomingCall)
	if len(out) != 1 || out[0].Provider != domain.ProviderAPNsVoIP {
		t.Fatalf("expected only apns_voip, got %+v", out)
	}
}

func TestFilterDevicesForMessagePrefersAPNsForNotification(t *testing.T) {
	devices := []*domain.Device{
		{ID: "1", AppID: "app", InstallationID: "inst1", Platform: domain.PlatformIOS, Provider: domain.ProviderAPNs},
		{ID: "2", AppID: "app", InstallationID: "inst1", Platform: domain.PlatformIOS, Provider: domain.ProviderAPNsVoIP},
	}
	out := filterDevicesForMessage(devices, domain.MessageNotification)
	if len(out) != 1 || out[0].Provider != domain.ProviderAPNs {
		t.Fatalf("expected only apns, got %+v", out)
	}
}
