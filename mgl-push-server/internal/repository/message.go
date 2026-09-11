package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
	"github.com/amjil/mgl-push/mgl-push-server/internal/idgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepository struct {
	db *pgxpool.Pool
}

func NewMessageRepository(db *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Create(ctx context.Context, m *domain.Message) (*domain.Message, error) {
	return r.create(ctx, r.db, m)
}

// CreateWithIdempotency atomically creates a message and binds an idempotency key
// in one transaction, respecting the FK from idempotency_keys → push_messages.
//
// On success: (createdMessage, true, nil).
// On duplicate key: (&domain.Message{ID: existingID}, false, nil).
func (r *MessageRepository) CreateWithIdempotency(ctx context.Context, m *domain.Message, key string) (*domain.Message, bool, error) {
	if key == "" {
		created, err := r.Create(ctx, m)
		return created, true, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)

	// Fast path / wait for in-flight commit: FOR SHARE blocks while another
	// transaction holds the row; empty result does not serialize newcomers.
	var existingID string
	err = tx.QueryRow(ctx, `
SELECT message_id FROM idempotency_keys
WHERE app_id = $1 AND idempotency_key = $2
FOR SHARE
`, m.AppID, key).Scan(&existingID)
	if err == nil {
		return &domain.Message{ID: existingID}, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}

	created, err := r.create(ctx, tx, m)
	if err != nil {
		return nil, false, err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO idempotency_keys (idempotency_key, app_id, message_id, created_at)
VALUES ($1, $2, $3, $4)
`, key, m.AppID, created.ID, time.Now().UTC())
	if err != nil {
		if isUniqueViolation(err) {
			// Concurrent winner committed; roll back our message insert and return theirs.
			_ = tx.Rollback(ctx)
			existingID, findErr := r.FindByIdempotencyKey(ctx, m.AppID, key)
			if findErr != nil {
				return nil, false, findErr
			}
			if existingID == "" {
				return nil, false, err
			}
			return &domain.Message{ID: existingID}, false, nil
		}
		return nil, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return created, true, nil
}

type dbQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *MessageRepository) create(ctx context.Context, q dbQuerier, m *domain.Message) (*domain.Message, error) {
	now := time.Now().UTC()
	if m.ID == "" {
		m.ID = idgen.New()
	}
	if m.Status == "" {
		m.Status = domain.MessageStatusCreated
	}
	if m.Priority == "" {
		m.Priority = domain.PriorityNormal
	}
	if m.Type == "" {
		m.Type = domain.MessageNotification
	}
	m.CreatedAt = now

	dataJSON, err := json.Marshal(m.Data)
	if err != nil {
		return nil, err
	}
	var ttlSeconds *int
	if m.TTL > 0 {
		s := int(m.TTL.Seconds())
		ttlSeconds = &s
	}

	const insertQ = `
INSERT INTO push_messages (
  id, app_id, type, title, body, data, image_url, priority, ttl_seconds,
  collapse_key, sound, badge, deep_link, category, status, created_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16
)
RETURNING id, app_id, type, title, body, data, image_url, priority, ttl_seconds,
  collapse_key, sound, badge, deep_link, category, status, created_at, queued_at, completed_at
`
	return scanMessage(q.QueryRow(ctx, insertQ,
		m.ID, m.AppID, string(m.Type), nullStr(m.Title), nullStr(m.Body), dataJSON, nullStr(m.ImageURL),
		m.Priority, ttlSeconds, nullStr(m.CollapseKey), nullStr(m.Sound), m.Badge,
		nullStr(m.DeepLink), nullStr(m.Category), m.Status, m.CreatedAt,
	))
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (r *MessageRepository) MarkQueued(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
UPDATE push_messages SET status = 'queued', queued_at = $2 WHERE id = $1
`, id, now)
	return err
}

func (r *MessageRepository) MarkCompleted(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
UPDATE push_messages SET status = 'completed', completed_at = $2 WHERE id = $1
`, id, now)
	return err
}

func (r *MessageRepository) MarkFailed(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
UPDATE push_messages SET status = 'failed', completed_at = $2 WHERE id = $1
`, id, now)
	return err
}

func (r *MessageRepository) FindByID(ctx context.Context, appID, id string) (*domain.Message, error) {
	var (
		row scannable
		err error
	)
	if appID == "" {
		const q = `
SELECT id, app_id, type, title, body, data, image_url, priority, ttl_seconds,
  collapse_key, sound, badge, deep_link, category, status, created_at, queued_at, completed_at
FROM push_messages WHERE id = $1
`
		row = r.db.QueryRow(ctx, q, id)
	} else {
		const q = `
SELECT id, app_id, type, title, body, data, image_url, priority, ttl_seconds,
  collapse_key, sound, badge, deep_link, category, status, created_at, queued_at, completed_at
FROM push_messages WHERE id = $1 AND app_id = $2
`
		row = r.db.QueryRow(ctx, q, id, appID)
	}
	m, err := scanMessage(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.MessageNotFound()
	}
	return m, err
}

func (r *MessageRepository) ClaimQueued(ctx context.Context, limit int) ([]*domain.Message, error) {
	const q = `
UPDATE push_messages SET status = 'sending'
WHERE id IN (
  SELECT id FROM push_messages
  WHERE status = 'queued'
  ORDER BY created_at
  LIMIT $1
  FOR UPDATE SKIP LOCKED
)
RETURNING id, app_id, type, title, body, data, image_url, priority, ttl_seconds,
  collapse_key, sound, badge, deep_link, category, status, created_at, queued_at, completed_at
`
	rows, err := r.db.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// FindByIdempotencyKey returns an existing message_id for a prior Idempotency-Key.
func (r *MessageRepository) FindByIdempotencyKey(ctx context.Context, appID, key string) (string, error) {
	var messageID string
	err := r.db.QueryRow(ctx, `
SELECT message_id FROM idempotency_keys WHERE app_id = $1 AND idempotency_key = $2
`, appID, key).Scan(&messageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return messageID, err
}

func scanMessage(row scannable) (*domain.Message, error) {
	var m domain.Message
	var msgType string
	var title, body, imageURL, collapseKey, sound, deepLink, category *string
	var dataJSON []byte
	var ttlSeconds *int
	var queuedAt, completedAt *time.Time
	err := row.Scan(
		&m.ID, &m.AppID, &msgType, &title, &body, &dataJSON, &imageURL, &m.Priority, &ttlSeconds,
		&collapseKey, &sound, &m.Badge, &deepLink, &category, &m.Status,
		&m.CreatedAt, &queuedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}
	m.Type = domain.MessageType(msgType)
	if m.Type == "" {
		m.Type = domain.MessageNotification
	}
	m.Title = deref(title)
	m.Body = deref(body)
	m.ImageURL = deref(imageURL)
	m.CollapseKey = deref(collapseKey)
	m.Sound = deref(sound)
	m.DeepLink = deref(deepLink)
	m.Category = deref(category)
	m.QueuedAt = queuedAt
	m.CompletedAt = completedAt
	if ttlSeconds != nil {
		m.TTL = time.Duration(*ttlSeconds) * time.Second
	}
	m.Data = map[string]string{}
	if len(dataJSON) > 0 {
		_ = json.Unmarshal(dataJSON, &m.Data)
	}
	return &m, nil
}
