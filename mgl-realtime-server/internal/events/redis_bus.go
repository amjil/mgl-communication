package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// UserTopic returns the Redis Pub/Sub channel for a user's device fanout.
func UserTopic(appID, userID string) string {
	return "user:" + appID + ":" + userID
}

// RedisBus is a distributed event bus backed by Redis Pub/Sub.
// Use Publish + SubscribeUserTopic for cross-node WebSocket routing.
type RedisBus struct {
	client *redis.Client
	logger *slog.Logger
}

func NewRedisBus(client *redis.Client, logger *slog.Logger) *RedisBus {
	if logger == nil {
		logger = slog.Default()
	}
	return &RedisBus{
		client: client,
		logger: logger,
	}
}

// Publish sends an Event to an explicit topic (routing key).
func (b *RedisBus) Publish(ctx context.Context, topic string, e Event) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return b.client.Publish(ctx, topic, data).Err()
}

// SubscribeUserTopic creates a subscription for user:{appID}:{userID}.
// The returned channel is buffered; cleanup unsubscribes and closes the Redis pubsub.
func (b *RedisBus) SubscribeUserTopic(ctx context.Context, appID, userID string) (<-chan Event, func()) {
	topic := UserTopic(appID, userID)
	pubsub := b.client.Subscribe(ctx, topic)

	ch := make(chan Event, 64)

	go func() {
		defer close(ch)
		for msg := range pubsub.Channel() {
			var e Event
			if err := json.Unmarshal([]byte(msg.Payload), &e); err != nil {
				b.logger.Warn("failed to decode pubsub event", "topic", topic, "error", err)
				continue
			}
			select {
			case ch <- e:
			case <-ctx.Done():
				return
			default:
				b.logger.Warn("pubsub channel full, dropping event", "topic", topic)
			}
		}
	}()

	cleanup := func() {
		_ = pubsub.Close()
	}
	return ch, cleanup
}

// Client exposes the underlying Redis client (e.g. for Ping / Close in main).
func (b *RedisBus) Client() *redis.Client {
	return b.client
}
