package presence

import (
	"context"
	"time"
)

const (
	StatusOffline = "offline"
	StatusOnline  = "online"
	StatusIdle    = "idle"
	StatusBusy    = "busy"
	StatusInCall  = "in_call"
)

type DevicePresence struct {
	DeviceID  string    `json:"device_id"`
	UserID    string    `json:"user_id"`
	AppID     string    `json:"app_id"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserPresence struct {
	UserID    string           `json:"user_id"`
	AppID     string           `json:"app_id"`
	Status    string           `json:"status"`
	Devices   []DevicePresence `json:"devices"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// Store 定义了用户与设备在线状态的存储接口
type Store interface {
	SetOnline(ctx context.Context, appID, userID, deviceID string) UserPresence
	SetOffline(ctx context.Context, appID, userID, deviceID string) UserPresence
	SetStatus(ctx context.Context, appID, userID, deviceID, status string) UserPresence
	GetUser(ctx context.Context, appID, userID string) UserPresence
}

func mergeStatus(a, b string) string {
	rank := map[string]int{
		StatusOffline: 0,
		StatusIdle:    1,
		StatusOnline:  2,
		StatusBusy:    3,
		StatusInCall:  4,
	}
	if rank[b] > rank[a] {
		return b
	}
	return a
}
