CREATE TABLE IF NOT EXISTS devices (
    id VARCHAR(26) PRIMARY KEY,
    user_id VARCHAR(255),
    installation_id VARCHAR(255) NOT NULL UNIQUE,
    platform VARCHAR(32) NOT NULL,
    provider VARCHAR(32) NOT NULL,
    token TEXT NOT NULL,
    app_id VARCHAR(255) NOT NULL,
    app_version VARCHAR(64),
    os_version VARCHAR(64),
    device_model VARCHAR(255),
    locale VARCHAR(32),
    timezone VARCHAR(64),
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_devices_user_id ON devices (user_id);
CREATE INDEX IF NOT EXISTS idx_devices_provider ON devices (provider);
CREATE INDEX IF NOT EXISTS idx_devices_token ON devices (token);
CREATE INDEX IF NOT EXISTS idx_devices_status ON devices (status);
CREATE INDEX IF NOT EXISTS idx_devices_app_id ON devices (app_id);
CREATE INDEX IF NOT EXISTS idx_devices_app_user ON devices (app_id, user_id) WHERE status = 'active';

CREATE TABLE IF NOT EXISTS push_messages (
    id VARCHAR(26) PRIMARY KEY,
    app_id VARCHAR(255) NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'notification',
    title TEXT,
    body TEXT,
    data JSONB,
    image_url TEXT,
    priority VARCHAR(32),
    ttl_seconds INTEGER,
    collapse_key VARCHAR(255),
    sound VARCHAR(64),
    badge INTEGER,
    deep_link TEXT,
    category VARCHAR(64),
    status VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    queued_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_push_messages_status ON push_messages (status);
CREATE INDEX IF NOT EXISTS idx_push_messages_app_id ON push_messages (app_id);
CREATE INDEX IF NOT EXISTS idx_push_messages_type ON push_messages (type);

CREATE TABLE IF NOT EXISTS push_deliveries (
    id VARCHAR(26) PRIMARY KEY,
    message_id VARCHAR(26) NOT NULL REFERENCES push_messages (id),
    device_id VARCHAR(26) NOT NULL REFERENCES devices (id),
    provider VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    provider_message_id TEXT,
    error_code VARCHAR(255),
    error_message TEXT,
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    opened_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_push_deliveries_message_id ON push_deliveries (message_id);
CREATE INDEX IF NOT EXISTS idx_push_deliveries_status ON push_deliveries (status);
CREATE INDEX IF NOT EXISTS idx_push_deliveries_ready ON push_deliveries (status, next_retry_at);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    idempotency_key VARCHAR(255) NOT NULL,
    app_id          VARCHAR(255) NOT NULL,
    message_id      VARCHAR(26)  NOT NULL REFERENCES push_messages (id),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (app_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_created ON idempotency_keys (created_at);
