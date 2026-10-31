-- 001_initial_schema.sql
-- APAAR Student Data Verification System Schema
-- Dev A: Schema & Tenancy Foundations

CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "fuzzystrmatch";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 1. Institutions
CREATE TABLE IF NOT EXISTS institutions (
    id VARCHAR(64) PRIMARY KEY,
    aicte_id VARCHAR(64) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    state VARCHAR(100) NOT NULL,
    type VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 2. Programs Reference
CREATE TABLE IF NOT EXISTS programs (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    department VARCHAR(255) NOT NULL,
    duration_years INT NOT NULL DEFAULT 4,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 3. Users (Role-based access: aicte_admin, institution_admin, student, verifier)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL CHECK (role IN ('aicte_admin', 'institution_admin', 'student', 'verifier')),
    institution_id VARCHAR(64) REFERENCES institutions(id) ON DELETE CASCADE,
    apaar_id VARCHAR(12),
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 4. Sessions (Hashed high-entropy session tokens)
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(128) PRIMARY KEY, -- SHA-256 hash of token
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL,
    institution_id VARCHAR(64),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 5. API Keys (Hashed API tokens for institutions)
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id VARCHAR(64) NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    key_hash VARCHAR(128) UNIQUE NOT NULL, -- SHA-256 hash
    key_prefix VARCHAR(16) NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{"upload:write", "upload:read"}',
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 6. Students (Identity ONLY; no institution ownership on the person)
CREATE TABLE IF NOT EXISTS students (
    apaar_id VARCHAR(12) PRIMARY KEY, -- Exactly 12 digits
    name VARCHAR(255) NOT NULL,
    dob DATE NOT NULL,
    gender VARCHAR(20),
    guardian_name VARCHAR(255),
    identity_status VARCHAR(50) NOT NULL DEFAULT 'pending_verification',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 7. Enrolments (Institution-owned academic relationship; multiple can coexist)
-- Rule #1: Unique on (institution_id, academic_year, admission_reference). NO global unique on (apaar_id, academic_year).
CREATE TABLE IF NOT EXISTS enrolments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    apaar_id VARCHAR(12) NOT NULL REFERENCES students(apaar_id) ON DELETE RESTRICT,
    institution_id VARCHAR(64) NOT NULL REFERENCES institutions(id) ON DELETE RESTRICT,
    program_id VARCHAR(64) NOT NULL REFERENCES programs(id) ON DELETE RESTRICT,
    academic_year VARCHAR(16) NOT NULL,
    admission_reference VARCHAR(128) NOT NULL,
    admission_date DATE,
    category VARCHAR(50) NOT NULL DEFAULT 'General',
    admission_type VARCHAR(50) NOT NULL DEFAULT 'Regular',
    is_supernumerary BOOLEAN NOT NULL DEFAULT FALSE,
    is_nri BOOLEAN NOT NULL DEFAULT FALSE,
    oci_status BOOLEAN NOT NULL DEFAULT FALSE,
    is_foreign_national BOOLEAN NOT NULL DEFAULT FALSE,
    nationality VARCHAR(64) NOT NULL DEFAULT 'INDIAN',
    status VARCHAR(50) NOT NULL DEFAULT 'submitted', -- submitted, verified, attested, flagged, rejected
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_enrolment_tenant_year_ref UNIQUE (institution_id, academic_year, admission_reference)
);

-- 8. Approved Intake (Unique per institution/program/year; serialized with locks)
CREATE TABLE IF NOT EXISTS approved_intake (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id VARCHAR(64) NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    program_id VARCHAR(64) NOT NULL REFERENCES programs(id) ON DELETE RESTRICT,
    academic_year VARCHAR(16) NOT NULL,
    regular_limit INT NOT NULL,
    applicable_allowances INT NOT NULL DEFAULT 0,
    policy_version VARCHAR(32) NOT NULL DEFAULT 'AICTE-2024-V1',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_intake_inst_prog_year UNIQUE (institution_id, program_id, academic_year)
);

-- 9. Upload Jobs (Job metadata, row counts and idempotency)
CREATE TABLE IF NOT EXISTS upload_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id VARCHAR(64) NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    created_by UUID REFERENCES users(id),
    filename VARCHAR(255) NOT NULL,
    total_rows INT NOT NULL DEFAULT 0,
    processed_rows INT NOT NULL DEFAULT 0,
    accepted_rows INT NOT NULL DEFAULT 0,
    flagged_rows INT NOT NULL DEFAULT 0,
    rejected_rows INT NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'queued', -- queued, processing, completed, failed
    idempotency_key VARCHAR(128),
    error_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_upload_idempotency UNIQUE (institution_id, idempotency_key)
);

-- 10. Upload Rows (Preserves raw payload and exactly one final processing outcome)
CREATE TABLE IF NOT EXISTS upload_rows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES upload_jobs(id) ON DELETE CASCADE,
    institution_id VARCHAR(64) NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    row_number INT NOT NULL,
    raw_payload JSONB NOT NULL,
    normalized_fields JSONB,
    parse_errors JSONB,
    outcome VARCHAR(50) NOT NULL DEFAULT 'staged', -- staged, accepted, flagged, rejected, parse_failed
    enrolment_id UUID REFERENCES enrolments(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_upload_row_job_num UNIQUE (job_id, row_number)
);

-- 11. Validation Flags (Review leads with explicit targets: upload_row, enrolment, or institution)
CREATE TABLE IF NOT EXISTS validation_flags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    upload_row_id UUID REFERENCES upload_rows(id) ON DELETE CASCADE,
    enrolment_id UUID REFERENCES enrolments(id) ON DELETE CASCADE,
    institution_id VARCHAR(64) NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    rule_code VARCHAR(32) NOT NULL, -- V1 - V9
    severity VARCHAR(32) NOT NULL, -- info, warning, error, conflict
    reason TEXT NOT NULL,
    evidence JSONB,
    policy_version VARCHAR(32) NOT NULL DEFAULT 'AICTE-2024-V1',
    review_status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, resolved, dismissed, escalated
    reviewer UUID REFERENCES users(id),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 12. Consents (Student granted verifier access, checked dynamically on every request)
CREATE TABLE IF NOT EXISTS consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_apaar_id VARCHAR(12) NOT NULL REFERENCES students(apaar_id) ON DELETE CASCADE,
    verifier_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    enrolment_id UUID REFERENCES enrolments(id) ON DELETE CASCADE,
    purpose VARCHAR(255) NOT NULL,
    permitted_fields TEXT[] NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, revoked, expired
    version INT NOT NULL DEFAULT 1,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

-- 13. Credentials (Ed25519 signed, opaque unguessable ID for QR landing)
CREATE TABLE IF NOT EXISTS credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    opaque_id VARCHAR(128) UNIQUE NOT NULL,
    enrolment_id UUID NOT NULL REFERENCES enrolments(id) ON DELETE RESTRICT,
    enrolment_version INT NOT NULL,
    canonical_payload JSONB NOT NULL,
    signature BYTEA NOT NULL,
    key_id VARCHAR(64) NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    supersedes_id UUID REFERENCES credentials(id) ON DELETE SET NULL
);

-- 14. Audit Events (Append-only hash-chained, tamper-evident log)
CREATE TABLE IF NOT EXISTS audit_events (
    id BIGSERIAL PRIMARY KEY,
    actor_id UUID,
    actor_role VARCHAR(50) NOT NULL,
    action VARCHAR(100) NOT NULL,
    target_type VARCHAR(50) NOT NULL,
    target_id VARCHAR(128) NOT NULL,
    institution_id VARCHAR(64),
    details JSONB,
    prev_hash VARCHAR(64) NOT NULL,
    event_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 15. Rollup Enrolment (Aggregates keyed by institution/program/year; no personal data)
CREATE TABLE IF NOT EXISTS rollup_enrolment (
    institution_id VARCHAR(64) NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    program_id VARCHAR(64) NOT NULL REFERENCES programs(id) ON DELETE RESTRICT,
    academic_year VARCHAR(16) NOT NULL,
    total_enrolled INT NOT NULL DEFAULT 0,
    supernumerary INT NOT NULL DEFAULT 0,
    nri INT NOT NULL DEFAULT 0,
    oci INT NOT NULL DEFAULT 0,
    foreign_national INT NOT NULL DEFAULT 0,
    flagged_count INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (institution_id, program_id, academic_year)
);

-- Indexes for performance, candidate blocking, and tenant isolation
CREATE INDEX IF NOT EXISTS idx_students_name_trgm ON students USING gin (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_students_dob_name_prefix ON students (dob, name);
CREATE INDEX IF NOT EXISTS idx_enrolments_institution_year ON enrolments (institution_id, academic_year);
CREATE INDEX IF NOT EXISTS idx_enrolments_apaar ON enrolments (apaar_id);
CREATE INDEX IF NOT EXISTS idx_upload_rows_job ON upload_rows (job_id, outcome);
CREATE INDEX IF NOT EXISTS idx_flags_institution_status ON validation_flags (institution_id, review_status);
CREATE INDEX IF NOT EXISTS idx_consents_student_verifier ON consents (student_apaar_id, verifier_id, status);
CREATE INDEX IF NOT EXISTS idx_credentials_opaque ON credentials (opaque_id);
CREATE INDEX IF NOT EXISTS idx_audit_chain_order ON audit_events (id ASC);
CREATE INDEX IF NOT EXISTS idx_audit_institution ON audit_events (institution_id, created_at);

