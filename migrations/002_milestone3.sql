-- 002_milestone3.sql
-- Milestone 3: Concurrency serialization, attestation tracking, and extended rollups

-- 1. Table for strictly serializing audit log appends via row-level locks
CREATE TABLE IF NOT EXISTS audit_head (
    id INT PRIMARY KEY DEFAULT 1,
    last_event_id BIGINT NOT NULL DEFAULT 0,
    last_event_hash VARCHAR(64) NOT NULL DEFAULT '0000000000000000000000000000000000000000000000000000000000000000',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT single_head_row CHECK (id = 1)
);

INSERT INTO audit_head (id, last_event_id, last_event_hash)
VALUES (1, 0, '0000000000000000000000000000000000000000000000000000000000000000')
ON CONFLICT (id) DO NOTHING;

-- 2. Extend enrolments with attestation tracking (Rule #8: attestation invalidated if version changes)
ALTER TABLE enrolments
    ADD COLUMN IF NOT EXISTS attested_version INT DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS attested_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS attested_at TIMESTAMPTZ DEFAULT NULL;

-- 3. Extend rollup_enrolment with held and resolved counts
ALTER TABLE rollup_enrolment
    ADD COLUMN IF NOT EXISTS held_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS flagged_resolved INT NOT NULL DEFAULT 0;

-- 4. Additional indexes for flag queries and reviews
CREATE INDEX IF NOT EXISTS idx_flags_rule_status ON validation_flags (rule_code, review_status);
CREATE INDEX IF NOT EXISTS idx_enrolments_attestation ON enrolments (institution_id, status, version);

-- 5. Revoke UPDATE and DELETE on audit_events to guarantee append-only ledger properties
REVOKE UPDATE, DELETE ON audit_events FROM PUBLIC;

