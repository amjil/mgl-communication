package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/repository"
)

var allowedProviders = map[string]bool{
	domain.ProviderHuawei: true,
	domain.ProviderXiaomi: true,
	domain.ProviderOppo:   true,
	domain.ProviderVivo:   true,
	domain.ProviderFCM:      true,
	domain.ProviderAPNs:     true,
	domain.ProviderAPNsVoIP: true,
}

var allowedPlatforms = map[string]bool{
	domain.PlatformAndroid: true,
	domain.PlatformIOS:     true,
}

type DeviceService struct {
	repo *repository.DeviceRepository
}

func NewDeviceService(repo *repository.DeviceRepository) *DeviceService {
	return &DeviceService{repo: repo}
}

type RegisterDeviceInput struct {
	InstallationID string
	UserID         string
	Platform       string
	Provider       string
	Token          string
	AppID          string
	AppVersion     string
	OSVersion      string
	DeviceModel    string
	Locale         string
	Timezone       string
}

func (s *DeviceService) Register(ctx context.Context, in RegisterDeviceInput) (*domain.Device, error) {
	if err := validateRegister(in); err != nil {
		return nil, err
	}
	d := &domain.Device{
		InstallationID: in.InstallationID,
		UserID:         in.UserID,
		Platform:       in.Platform,
		Provider:       in.Provider,
		Token:          in.Token,
		AppID:          in.AppID,
		AppVersion:     in.AppVersion,
		OSVersion:      in.OSVersion,
		DeviceModel:    in.DeviceModel,
		Locale:         in.Locale,
		Timezone:       in.Timezone,
		Status:         domain.DeviceStatusActive,
	}
	return s.repo.Upsert(ctx, d)
}

func (s *DeviceService) UpdateToken(ctx context.Context, appID, installationID, provider, token string) (*domain.Device, error) {
	if installationID == "" || token == "" {
		return nil, domain.InvalidRequest("installation_id and token are required")
	}
	if provider != "" && !allowedProviders[provider] {
		return nil, domain.InvalidRequest("invalid provider")
	}
	return s.repo.UpdateToken(ctx, appID, installationID, provider, token)
}

func (s *DeviceService) SetUserID(ctx context.Context, appID, installationID, userID string) error {
	if installationID == "" || userID == "" {
		return domain.InvalidRequest("installation_id and user_id are required")
	}
	return s.repo.SetUserID(ctx, appID, installationID, userID)
}

func (s *DeviceService) ClearUserID(ctx context.Context, appID, installationID string) error {
	if installationID == "" {
		return domain.InvalidRequest("installation_id is required")
	}
	return s.repo.ClearUserID(ctx, appID, installationID)
}

func (s *DeviceService) Unregister(ctx context.Context, appID, installationID string) error {
	if installationID == "" {
		return domain.InvalidRequest("installation_id is required")
	}
	return s.repo.Unregister(ctx, appID, installationID)
}

func (s *DeviceService) Get(ctx context.Context, appID, installationID string) (*domain.Device, error) {
	return s.repo.FindByInstallationID(ctx, appID, installationID)
}

func validateRegister(in RegisterDeviceInput) error {
	if in.InstallationID == "" {
		return domain.InvalidRequest("installation_id is required")
	}
	if in.Platform == "" || !allowedPlatforms[in.Platform] {
		return domain.InvalidRequest("platform must be android or ios")
	}
	if in.Provider == "" || !allowedProviders[in.Provider] {
		return domain.InvalidRequest("invalid provider")
	}
	if in.Token == "" {
		return domain.InvalidRequest("token is required")
	}
	if in.AppID == "" {
		return domain.InvalidRequest("app_id is required")
	}
	if utf8.RuneCountInString(in.Token) > 4096 {
		return domain.InvalidRequest("token too long")
	}
	return nil
}

func MaskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:5] + "..." + token[len(token)-3:]
}

func NormalizeProvider(p string) string {
	return strings.ToLower(strings.TrimSpace(p))
}
