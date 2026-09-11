-- Spec 2.1: message type + idempotency

ALTER TABLE push_messages
    ADD COLUMN IF NOT EXISTS type VARCHAR(32) NOT NULL DEFAULT 'notification';

CREATE INDEX IF NOT EXISTS idx_push_messages_type ON push_messages (type);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    idempotency_key VARCHAR(255) NOT NULL,
    app_id          VARCHAR(255) NOT NULL,
    message_id      VARCHAR(26)  NOT NULL REFERENCES push_messages (id),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (app_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_created ON idempotency_keys (created_at);

ALTER TABLE push_deliveries
    ADD COLUMN IF NOT EXISTS accepted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS failed_at TIMESTAMPTZ;
