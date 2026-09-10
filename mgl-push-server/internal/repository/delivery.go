package repository

import (
	"context"
	"time"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
	"github.com/amjil/mgl-push/mgl-push-server/internal/idgen"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeliveryRepository struct {
	db *pgxpool.Pool
}

func NewDeliveryRepository(db *pgxpool.Pool) *DeliveryRepository {
	return &DeliveryRepository{db: db}
}

func (r *DeliveryRepository) CreateMany(ctx context.Context, deliveries []*domain.Delivery) error {
	if len(deliveries) == 0 {
		return nil
	}
	now := time.Now().UTC()
	for _, d := range deliveries {
		if d.ID == "" {
			d.ID = idgen.New()
		}
		if d.Status == "" {
			d.Status = domain.DeliveryStatusPending
		}
		d.CreatedAt = now
		_, err := r.db.Exec(ctx, `
INSERT INTO push_deliveries (
  id, message_id, device_id, provider, status, attempts, created_at, next_retry_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
`, d.ID, d.MessageID, d.DeviceID, d.Provider, d.Status, d.Attempts, d.CreatedAt, d.NextRetryAt)
		if err != nil {
			return err
		}
	}
	return nil
}
func (r *DeliveryRepository) ClaimReady(ctx context.Context, limit int) ([]*domain.Delivery, error) {
	now := time.Now().UTC()
	const q = `
UPDATE push_deliveries SET status = 'sending', attempts = attempts + 1
WHERE id IN (
  SELECT id FROM push_deliveries
  WHERE status IN ('pending', 'retrying')
    AND (next_retry_at IS NULL OR next_retry_at <= $1)
  ORDER BY created_at
  LIMIT $2
  FOR UPDATE SKIP LOCKED
)
RETURNING id, message_id, device_id, provider, status, provider_message_id,
  error_code, error_message, attempts, created_at, sent_at, delivered_at, opened_at, next_retry_at
`
	rows, err := r.db.Query(ctx, q, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Delivery
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *DeliveryRepository) MarkAccepted(ctx context.Context, id, providerMessageID string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
UPDATE push_deliveries SET status = 'accepted', provider_message_id = $2,
  sent_at = $3, error_code = NULL, error_message = NULL, next_retry_at = NULL
WHERE id = $1
`, id, providerMessageID, now)
	return err
}

func (r *DeliveryRepository) MarkFailed(ctx context.Context, id, code, message string) error {
	_, err := r.db.Exec(ctx, `
UPDATE push_deliveries SET status = 'failed', error_code = $2, error_message = $3, next_retry_at = NULL
WHERE id = $1
`, id, code, message)
	return err
}

func (r *DeliveryRepository) MarkRetrying(ctx context.Context, id, code, message string, nextRetryAt time.Time) error {
	_, err := r.db.Exec(ctx, `
UPDATE push_deliveries SET status = 'retrying', error_code = $2, error_message = $3, next_retry_at = $4
WHERE id = $1
`, id, code, message, nextRetryAt)
	return err
}

func (r *DeliveryRepository) CountByMessage(ctx context.Context, messageID string) (total, terminal int, err error) {
	err = r.db.QueryRow(ctx, `
SELECT
  COUNT(*),
  COUNT(*) FILTER (WHERE status IN ('accepted', 'failed', 'delivered', 'opened'))
FROM push_deliveries WHERE message_id = $1
`, messageID).Scan(&total, &terminal)
	return
}

func (r *DeliveryRepository) HasPending(ctx context.Context, messageID string) (bool, error) {
	var n int
	err := r.db.QueryRow(ctx, `
SELECT COUNT(*) FROM push_deliveries
WHERE message_id = $1 AND status IN ('pending', 'sending', 'retrying')
`, messageID).Scan(&n)
	return n > 0, err
}

func scanDelivery(row scannable) (*domain.Delivery, error) {
	var d domain.Delivery
	var providerMsgID, errCode, errMsg *string
	var sentAt, deliveredAt, openedAt, nextRetryAt *time.Time
	err := row.Scan(
		&d.ID, &d.MessageID, &d.DeviceID, &d.Provider, &d.Status, &providerMsgID,
		&errCode, &errMsg, &d.Attempts, &d.CreatedAt, &sentAt, &deliveredAt, &openedAt, &nextRetryAt,
	)
	if err != nil {
		return nil, err
	}
	d.ProviderMessageID = deref(providerMsgID)
	d.ErrorCode = deref(errCode)
	d.ErrorMessage = deref(errMsg)
	d.SentAt = sentAt
	d.DeliveredAt = deliveredAt
	d.OpenedAt = openedAt
	d.NextRetryAt = nextRetryAt
	return &d, nil
}
