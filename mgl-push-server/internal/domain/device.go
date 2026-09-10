package domain

import "time"

const (
	PlatformAndroid = "android"
	PlatformIOS     = "ios"

	ProviderHuawei = "huawei"
	ProviderXiaomi = "xiaomi"
	ProviderOppo   = "oppo"
	ProviderVivo   = "vivo"
	ProviderFCM    = "fcm"
	ProviderAPNs   = "apns"

	DeviceStatusActive   = "active"
	DeviceStatusInvalid  = "invalid"
	DeviceStatusDisabled = "disabled"
)

type Device struct {
	ID             string
	UserID         string
	InstallationID string
	Platform       string
	Provider       string
	Token          string
	AppID          string
	AppVersion     string
	OSVersion      string
	DeviceModel    string
	Locale         string
	Timezone       string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LastSeenAt     time.Time
}
