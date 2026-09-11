package presence

import (
	"context"
	"encoding/json"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/events"
	"github.com/redis/go-redis/v9"
)

const (
	presenceDevicePrefix = "presence:dev:"
	presenceUserPrefix   = "presence:usr:"
)

type RedisStore struct {
	client *redis.Client
	bus    *events.Bus
	ttl    time.Duration
}

func NewRedisStore(client *redis.Client, bus *events.Bus, ttl time.Duration) *RedisStore {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &RedisStore{
		client: client,
		bus:    bus,
		ttl:    ttl,
	}
}

func (s *RedisStore) SetOnline(ctx context.Context, appID, userID, deviceID string) UserPresence {
	return s.update(ctx, appID, userID, deviceID, StatusOnline, events.PresenceOnline)
}

func (s *RedisStore) SetOffline(ctx context.Context, appID, userID, deviceID string) UserPresence {
	return s.update(ctx, appID, userID, deviceID, StatusOffline, events.PresenceOffline)
}

func (s *RedisStore) SetStatus(ctx context.Context, appID, userID, deviceID, status string) UserPresence {
	return s.update(ctx, appID, userID, deviceID, status, events.PresenceUpdated)
}

func (s *RedisStore) update(ctx context.Context, appID, userID, deviceID, status, eventType string) UserPresence {
	now := time.Now().UTC()
	dp := DevicePresence{
		DeviceID:  deviceID,
		UserID:    userID,
		AppID:     appID,
		Status:    status,
		UpdatedAt: now,
	}
	data, _ := json.Marshal(dp)

	dKey := presenceDevicePrefix + deviceID
	uKey := presenceUserPrefix + appID + ":" + userID

	// 使用 Pipeline 原子化写入 Device 详情并更新 User 索引
	pipe := s.client.Pipeline()
	pipe.Set(ctx, dKey, data, s.ttl)
	pipe.SAdd(ctx, uKey, deviceID)
	pipe.Expire(ctx, uKey, s.ttl)
	_, _ = pipe.Exec(ctx)

	// 立即拉取聚合后的最新状态
	up := s.GetUser(ctx, appID, userID)

	// 本地 Bus 广播事件（为了兼容单节点或监控需求）
	if s.bus != nil {
		s.bus.Publish(events.Event{
			Type:     eventType,
			AppID:    appID,
			UserID:   userID,
			DeviceID: deviceID,
			Payload: map[string]any{
				"status": up.Status,
			},
		})
	}
	return up
}

func (s *RedisStore) GetUser(ctx context.Context, appID, userID string) UserPresence {
	uKey := presenceUserPrefix + appID + ":" + userID

	// 1. 获取该用户的所有活跃设备 ID
	deviceIDs, err := s.client.SMembers(ctx, uKey).Result()
	if err != nil || len(deviceIDs) == 0 {
		return UserPresence{UserID: userID, AppID: appID, Status: StatusOffline, UpdatedAt: time.Now().UTC()}
	}

	// 2. 批量拉取设备详情
	keys := make([]string, len(deviceIDs))
	for i, id := range deviceIDs {
		keys[i] = presenceDevicePrefix + id
	}

	vals, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return UserPresence{UserID: userID, AppID: appID, Status: StatusOffline, UpdatedAt: time.Now().UTC()}
	}

	var devices []DevicePresence
	var toRemove []string
	agg := StatusOffline
	var updated time.Time

	for i, val := range vals {
		devID := deviceIDs[i]
		if val == nil {
			toRemove = append(toRemove, devID) // 惰性清理过期设备
			continue
		}

		strVal, ok := val.(string)
		if !ok {
			continue
		}

		var dp DevicePresence
		if err := json.Unmarshal([]byte(strVal), &dp); err == nil {
			devices = append(devices, dp)
			agg = mergeStatus(agg, dp.Status)
			if dp.UpdatedAt.After(updated) {
				updated = dp.UpdatedAt
			}
		}
	}

	if len(toRemove) > 0 {
		args := make([]any, len(toRemove))
		for i, v := range toRemove {
			args[i] = v
		}
		// 异步清理失效的 DeviceID 索引
		go s.client.SRem(context.Background(), uKey, args...)
	}

	if updated.IsZero() {
		updated = time.Now().UTC()
	}

	return UserPresence{
		UserID:    userID,
		AppID:     appID,
		Status:    agg,
		Devices:   devices,
		UpdatedAt: updated,
	}
}
