-- 003_milestone4.sql
-- Milestone 4: Cross-institution checks (V7), Fuzzy matching (V8), Institution anomaly (V9)
-- Review workflow backend & historical baseline

CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "fuzzystrmatch";

-- Trigram index on students name for bounded fuzzy candidate matching
CREATE INDEX IF NOT EXISTS idx_students_name_trgm ON students USING gin (name gin_trgm_ops);

-- Candidate blocking index on (dob, name)
CREATE INDEX IF NOT EXISTS idx_students_dob_name_prefix ON students (dob, name);

-- Review workflow fields on validation_flags
ALTER TABLE validation_flags ADD COLUMN IF NOT EXISTS review_decision VARCHAR(50);
ALTER TABLE validation_flags ADD COLUMN IF NOT EXISTS decision_note TEXT;
ALTER TABLE validation_flags ADD COLUMN IF NOT EXISTS decided_at TIMESTAMPTZ;

-- Index for review queue queries
CREATE INDEX IF NOT EXISTS idx_flags_rule_review_status ON validation_flags (rule_code, review_status);

-- Institution history for historical anomaly detection (Rule V9)
CREATE TABLE IF NOT EXISTS institution_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id VARCHAR(64) NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    academic_year VARCHAR(16) NOT NULL,
    total_enrolment INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_institution_history_year UNIQUE (institution_id, academic_year)
);

CREATE INDEX IF NOT EXISTS idx_institution_history_inst ON institution_history (institution_id);

