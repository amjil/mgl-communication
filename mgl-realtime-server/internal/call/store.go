package call

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
	"github.com/redis/go-redis/v9"
)

const (
	callKeyPrefix   = "call:"
	userCallsPrefix = "user_calls:"
	callExpiry      = 24 * time.Hour // 24小时兜底过期，防止僵尸数据
)

// Store 定义了通话状态的存储接口
type Store interface {
	Put(ctx context.Context, c *Call) error
	Get(ctx context.Context, callID string) (*Call, error)
	Update(ctx context.Context, callID string, fn func(*Call) error) (*Call, error)
	ActiveCallIDsForUser(ctx context.Context, appID, userID string) []string
}

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

// Put 插入新通话，并更新参与者的活跃索引
func (s *RedisStore) Put(ctx context.Context, c *Call) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	pipe := s.client.Pipeline()
	pipe.Set(ctx, callKeyPrefix+c.ID, data, callExpiry)

	for _, p := range c.Participants {
		uk := userCallsPrefix + c.AppID + ":" + p.UserID
		pipe.SAdd(ctx, uk, c.ID)
		pipe.Expire(ctx, uk, callExpiry)
	}

	_, err = pipe.Exec(ctx)
	return err
}

// Get 获取单条通话记录
func (s *RedisStore) Get(ctx context.Context, callID string) (*Call, error) {
	val, err := s.client.Get(ctx, callKeyPrefix+callID).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, domain.CallNotFound()
		}
		return nil, err
	}

	var c Call
	if err := json.Unmarshal([]byte(val), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Update 提供基于乐观锁 (WATCH) 的并发安全更新机制
func (s *RedisStore) Update(ctx context.Context, callID string, fn func(*Call) error) (*Call, error) {
	key := callKeyPrefix + callID
	var updatedCall *Call

	// 乐观锁重试循环 (最大重试 20 次)
	const maxRetries = 20
	for i := 0; i < maxRetries; i++ {
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			// 1. 读取当前最新状态
			val, err := tx.Get(ctx, key).Result()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					return domain.CallNotFound()
				}
				return err
			}

			var c Call
			if err := json.Unmarshal([]byte(val), &c); err != nil {
				return err
			}

			// 2. 执行业务修改逻辑 (比如 Accept, Reject)
			if err := fn(&c); err != nil {
				return err
			}

			// 3. 序列化修改后的数据
			updatedBytes, err := json.Marshal(&c)
			if err != nil {
				return err
			}

			// 4. 事务提交：如果在 Watch 期间 key 被其他节点修改了，TxPipelined 会返回 redis.TxFailedErr
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, key, updatedBytes, callExpiry)

				// 补偿逻辑：如果业务逻辑中加入了新参与者 (例如 Join)，需要同步更新 Set
				for _, p := range c.Participants {
					uk := userCallsPrefix + c.AppID + ":" + p.UserID
					pipe.SAdd(ctx, uk, c.ID)
					pipe.Expire(ctx, uk, callExpiry)
				}
				return nil
			})

			if err == nil {
				updatedCall = &c
			}
			return err
		}, key)

		if errors.Is(err, redis.TxFailedErr) {
			// 发生并发冲突，继续下一轮重试
			continue
		}
		if err != nil {
			return nil, err
		}
		// 成功
		return updatedCall, nil
	}

	return nil, domain.Internal("call update failed due to high concurrency")
}

// ActiveCallIDsForUser 获取用户的活跃通话，并执行惰性清理
func (s *RedisStore) ActiveCallIDsForUser(ctx context.Context, appID, userID string) []string {
	uk := userCallsPrefix + appID + ":" + userID
	callIDs, err := s.client.SMembers(ctx, uk).Result()
	if err != nil || len(callIDs) == 0 {
		return nil
	}

	var active []string
	var toRemove []string

	// 使用 MGet 批量拉取所有 Call 状态，优化网络 I/O
	keys := make([]string, len(callIDs))
	for i, id := range callIDs {
		keys[i] = callKeyPrefix + id
	}

	vals, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil
	}

	for i, val := range vals {
		id := callIDs[i]
		if val == nil {
			// 数据已过期或被删除
			toRemove = append(toRemove, id)
			continue
		}

		strVal, ok := val.(string)
		if !ok {
			continue
		}

		var c Call
		if err := json.Unmarshal([]byte(strVal), &c); err != nil {
			continue
		}

		if !c.IsTerminal() {
			active = append(active, id)
		} else {
			toRemove = append(toRemove, id)
		}
	}

	// 惰性清理 (Lazy Cleanup)：将已经完结或消失的通话从用户的活跃集合中剔除
	if len(toRemove) > 0 {
		args := make([]any, len(toRemove))
		for i, v := range toRemove {
			args[i] = v
		}
		// 异步执行，不阻塞当前查询返回
		go s.client.SRem(context.Background(), uk, args...)
	}

	return active
}
