package repository

import (
	"context"
	"errors"
	"time"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
	"github.com/amjil/mgl-push/mgl-push-server/internal/idgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeviceRepository struct {
	db *pgxpool.Pool
}

func NewDeviceRepository(db *pgxpool.Pool) *DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) Upsert(ctx context.Context, d *domain.Device) (*domain.Device, error) {
	now := time.Now().UTC()
	if d.ID == "" {
		d.ID = idgen.New()
	}
	if d.Status == "" {
		d.Status = domain.DeviceStatusActive
	}
	d.CreatedAt = now
	d.UpdatedAt = now
	d.LastSeenAt = now

	const q = `
INSERT INTO devices (
  id, user_id, installation_id, platform, provider, token,
  app_id, app_version, os_version, device_model, locale, timezone,
  status, created_at, updated_at, last_seen_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16
)
ON CONFLICT (installation_id) DO UPDATE SET
  user_id = COALESCE(EXCLUDED.user_id, devices.user_id),
  platform = EXCLUDED.platform,
  provider = EXCLUDED.provider,
  token = EXCLUDED.token,
  app_id = EXCLUDED.app_id,
  app_version = EXCLUDED.app_version,
  os_version = EXCLUDED.os_version,
  device_model = EXCLUDED.device_model,
  locale = EXCLUDED.locale,
  timezone = EXCLUDED.timezone,
  status = 'active',
  updated_at = EXCLUDED.updated_at,
  last_seen_at = EXCLUDED.last_seen_at
RETURNING id, user_id, installation_id, platform, provider, token,
  app_id, app_version, os_version, device_model, locale, timezone,
  status, created_at, updated_at, last_seen_at
`
	row := r.db.QueryRow(ctx, q,
		d.ID, nullStr(d.UserID), d.InstallationID, d.Platform, d.Provider, d.Token,
		d.AppID, nullStr(d.AppVersion), nullStr(d.OSVersion), nullStr(d.DeviceModel),
		nullStr(d.Locale), nullStr(d.Timezone), d.Status, d.CreatedAt, d.UpdatedAt, d.LastSeenAt,
	)
	return scanDevice(row)
}

func (r *DeviceRepository) FindByInstallationID(ctx context.Context, appID, installationID string) (*domain.Device, error) {
	const q = `
SELECT id, user_id, installation_id, platform, provider, token,
  app_id, app_version, os_version, device_model, locale, timezone,
  status, created_at, updated_at, last_seen_at
FROM devices
WHERE installation_id = $1 AND app_id = $2
`
	d, err := scanDevice(r.db.QueryRow(ctx, q, installationID, appID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.DeviceNotFound()
	}
	return d, err
}

func (r *DeviceRepository) UpdateToken(ctx context.Context, appID, installationID, provider, token string) (*domain.Device, error) {
	const q = `
UPDATE devices SET provider = $3, token = $4, status = 'active',
  updated_at = $5, last_seen_at = $5
WHERE installation_id = $1 AND app_id = $2
RETURNING id, user_id, installation_id, platform, provider, token,
  app_id, app_version, os_version, device_model, locale, timezone,
  status, created_at, updated_at, last_seen_at
`
	d, err := scanDevice(r.db.QueryRow(ctx, q, installationID, appID, provider, token, time.Now().UTC()))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.DeviceNotFound()
	}
	return d, err
}

func (r *DeviceRepository) SetUserID(ctx context.Context, appID, installationID, userID string) error {
	tag, err := r.db.Exec(ctx, `
UPDATE devices SET user_id = $3, updated_at = $4, last_seen_at = $4
WHERE installation_id = $1 AND app_id = $2
`, installationID, appID, userID, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.DeviceNotFound()
	}
	return nil
}

func (r *DeviceRepository) ClearUserID(ctx context.Context, appID, installationID string) error {
	tag, err := r.db.Exec(ctx, `
UPDATE devices SET user_id = NULL, updated_at = $3, last_seen_at = $3
WHERE installation_id = $1 AND app_id = $2
`, installationID, appID, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.DeviceNotFound()
	}
	return nil
}

func (r *DeviceRepository) Unregister(ctx context.Context, appID, installationID string) error {
	tag, err := r.db.Exec(ctx, `
UPDATE devices SET status = 'disabled', updated_at = $3
WHERE installation_id = $1 AND app_id = $2
`, installationID, appID, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.DeviceNotFound()
	}
	return nil
}

func (r *DeviceRepository) MarkInvalid(ctx context.Context, deviceID string) error {
	_, err := r.db.Exec(ctx, `
UPDATE devices SET status = 'invalid', updated_at = $2 WHERE id = $1
`, deviceID, time.Now().UTC())
	return err
}

func (r *DeviceRepository) ListActiveByUserIDs(ctx context.Context, appID string, userIDs []string) ([]*domain.Device, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	const q = `
SELECT id, user_id, installation_id, platform, provider, token,
  app_id, app_version, os_version, device_model, locale, timezone,
  status, created_at, updated_at, last_seen_at
FROM devices
WHERE app_id = $1 AND status = 'active' AND user_id = ANY($2)
`
	rows, err := r.db.Query(ctx, q, appID, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDevices(rows)
}

func (r *DeviceRepository) ListActiveByInstallationIDs(ctx context.Context, appID string, ids []string) ([]*domain.Device, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	const q = `
SELECT id, user_id, installation_id, platform, provider, token,
  app_id, app_version, os_version, device_model, locale, timezone,
  status, created_at, updated_at, last_seen_at
FROM devices
WHERE app_id = $1 AND status = 'active' AND installation_id = ANY($2)
`
	rows, err := r.db.Query(ctx, q, appID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDevices(rows)
}

func (r *DeviceRepository) FindByID(ctx context.Context, id string) (*domain.Device, error) {
	const q = `
SELECT id, user_id, installation_id, platform, provider, token,
  app_id, app_version, os_version, device_model, locale, timezone,
  status, created_at, updated_at, last_seen_at
FROM devices WHERE id = $1
`
	d, err := scanDevice(r.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.DeviceNotFound()
	}
	return d, err
}

type scannable interface {
	Scan(dest ...any) error
}

func scanDevice(row scannable) (*domain.Device, error) {
	var d domain.Device
	var userID, appVersion, osVersion, deviceModel, locale, timezone *string
	var lastSeen *time.Time
	err := row.Scan(
		&d.ID, &userID, &d.InstallationID, &d.Platform, &d.Provider, &d.Token,
		&d.AppID, &appVersion, &osVersion, &deviceModel, &locale, &timezone,
		&d.Status, &d.CreatedAt, &d.UpdatedAt, &lastSeen,
	)
	if err != nil {
		return nil, err
	}
	d.UserID = deref(userID)
	d.AppVersion = deref(appVersion)
	d.OSVersion = deref(osVersion)
	d.DeviceModel = deref(deviceModel)
	d.Locale = deref(locale)
	d.Timezone = deref(timezone)
	if lastSeen != nil {
		d.LastSeenAt = *lastSeen
	}
	return &d, nil
}

func scanDevices(rows pgx.Rows) ([]*domain.Device, error) {
	var out []*domain.Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
