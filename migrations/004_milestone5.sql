-- 004_milestone5.sql
-- Milestone 5: Scoped API Keys, Webhook Deliveries, Reports & Lifecycle

-- 1. Extend api_keys for verifiers and usage tracking
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS verifier_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ;
ALTER TABLE api_keys ALTER COLUMN institution_id DROP NOT NULL;
ALTER TABLE credentials ADD COLUMN IF NOT EXISTS revocation_reason TEXT;

-- 2. Webhook deliveries table for signed flag events and delivery inspection
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID UNIQUE NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    institution_id VARCHAR(64),
    flag_id UUID,
    rule_code VARCHAR(32),
    severity VARCHAR(32),
    payload JSONB NOT NULL,
    signature VARCHAR(128) NOT NULL,
    attempts INT NOT NULL DEFAULT 1,
    status VARCHAR(32) NOT NULL DEFAULT 'delivered', -- delivered, failed, retrying
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries (status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_event ON webhook_deliveries (event_id);
