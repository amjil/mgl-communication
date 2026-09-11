package oplog

import (
	"context"
	"encoding/json"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/signaling/protocol"
	"github.com/redis/go-redis/v9"
)

const maxOplogSize = 200 // 保留单次通话最新的 200 条瞬态信令

// Store 定义了通话信令的增量日志接口
type Store interface {
	Append(ctx context.Context, callID string, env protocol.Envelope) error
	GetRecent(ctx context.Context, callID string) ([]protocol.Envelope, error)
}

// RedisStore 集群模式下的 Oplog 实现
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisStore(client *redis.Client, ttl time.Duration) *RedisStore {
	if ttl <= 0 {
		ttl = 3 * time.Minute // Oplog 的有效期通常略长于预期的断网恢复时间
	}
	return &RedisStore{client: client, ttl: ttl}
}

func (s *RedisStore) Append(ctx context.Context, callID string, env protocol.Envelope) error {
	key := "oplog:call:" + callID
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}

	pipe := s.client.Pipeline()
	pipe.RPush(ctx, key, data)
	pipe.LTrim(ctx, key, -maxOplogSize, -1) // 剔除超出的旧信令，防止内存无限增长
	pipe.Expire(ctx, key, s.ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) GetRecent(ctx context.Context, callID string) ([]protocol.Envelope, error) {
	key := "oplog:call:" + callID
	vals, err := s.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	var out []protocol.Envelope
	for _, val := range vals {
		var env protocol.Envelope
		if err := json.Unmarshal([]byte(val), &env); err == nil {
			out = append(out, env)
		}
	}
	return out, nil
}

// NoOpStore 单机开发模式下的兜底，避免引入复杂性
type NoOpStore struct{}

func NewNoOpStore() *NoOpStore { return &NoOpStore{} }

func (s *NoOpStore) Append(ctx context.Context, callID string, env protocol.Envelope) error {
	return nil
}

func (s *NoOpStore) GetRecent(ctx context.Context, callID string) ([]protocol.Envelope, error) {
	return nil, nil
}
