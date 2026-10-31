package models

import (
	"time"

	"github.com/google/uuid"
)

// Role definitions
const (
	RoleAICTEAdmin       = "aicte_admin"
	RoleInstitutionAdmin = "institution_admin"
	RoleStudent          = "student"
	RoleVerifier         = "verifier"
)

// Upload and processing statuses
const (
	UploadStatusQueued     = "queued"
	UploadStatusProcessing = "processing"
	UploadStatusCompleted  = "completed"
	UploadStatusFailed     = "failed"

	RowOutcomeStaged      = "staged"
	RowOutcomeAccepted    = "accepted"
	RowOutcomeFlagged     = "flagged"
	RowOutcomeRejected    = "rejected"
	RowOutcomeParseFailed = "parse_failed"

	FlagSeverityInfo     = "info"
	FlagSeverityWarning  = "warning"
	FlagSeverityError    = "error"
	FlagSeverityConflict = "conflict"

	FlagStatusPending   = "pending"
	FlagStatusResolved  = "resolved"
	FlagStatusDismissed = "dismissed"
	FlagStatusEscalated = "escalated"

	ConsentStatusActive  = "active"
	ConsentStatusRevoked = "revoked"
	ConsentStatusExpired = "expired"

	EnrolmentStatusSubmitted = "submitted"
	EnrolmentStatusVerified  = "verified"
	EnrolmentStatusAttested  = "attested"
	EnrolmentStatusFlagged   = "flagged"
	EnrolmentStatusRejected  = "rejected"
)

// Institution represents an educational institution
type Institution struct {
	ID        string    `json:"id" db:"id"`
	AICTEID   string    `json:"aicte_id" db:"aicte_id"`
	Name      string    `json:"name" db:"name"`
	State     string    `json:"state" db:"state"`
	Type      string    `json:"type" db:"type"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Program represents an academic degree/program reference
type Program struct {
	ID            string    `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	Department    string    `json:"department" db:"department"`
	DurationYears int       `json:"duration_years" db:"duration_years"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// User represents an authenticated account
type User struct {
	ID            uuid.UUID `json:"id" db:"id"`
	Email         string    `json:"email" db:"email"`
	PasswordHash  string    `json:"-" db:"password_hash"`
	Role          string    `json:"role" db:"role"`
	InstitutionID *string   `json:"institution_id,omitempty" db:"institution_id"`
	APAARID       *string   `json:"apaar_id,omitempty" db:"apaar_id"`
	Status        string    `json:"status" db:"status"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// Session represents an active browser session with SHA-256 hashed token
type Session struct {
	ID            string    `json:"id" db:"id"` // SHA-256 of raw session token
	UserID        uuid.UUID `json:"user_id" db:"user_id"`
	Role          string    `json:"role" db:"role"`
	InstitutionID *string   `json:"institution_id,omitempty" db:"institution_id"`
	ExpiresAt     time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// Student represents an individual identity ONLY (no institution ownership on person)
type Student struct {
	APAARID        string    `json:"apaar_id" db:"apaar_id"`
	Name           string    `json:"name" db:"name"`
	DOB            time.Time `json:"dob" db:"dob"`
	Gender         *string   `json:"gender,omitempty" db:"gender"`
	GuardianName   *string   `json:"guardian_name,omitempty" db:"guardian_name"`
	IdentityStatus string    `json:"identity_status" db:"identity_status"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// Enrolment represents an academic record owned by a specific institution
type Enrolment struct {
	ID                 uuid.UUID  `json:"id" db:"id"`
	APAARID            string     `json:"apaar_id" db:"apaar_id"`
	InstitutionID      string     `json:"institution_id" db:"institution_id"`
	ProgramID          string     `json:"program_id" db:"program_id"`
	AcademicYear       string     `json:"academic_year" db:"academic_year"`
	AdmissionReference string     `json:"admission_reference" db:"admission_reference"`
	AdmissionDate      *time.Time `json:"admission_date,omitempty" db:"admission_date"`
	Category           string     `json:"category" db:"category"`
	AdmissionType      string     `json:"admission_type" db:"admission_type"`
	IsSupernumerary    bool       `json:"is_supernumerary" db:"is_supernumerary"`
	IsNRI              bool       `json:"is_nri" db:"is_nri"`
	OCIStatus          bool       `json:"oci_status" db:"oci_status"`
	IsForeignNational  bool       `json:"is_foreign_national" db:"is_foreign_national"`
	Nationality        string     `json:"nationality" db:"nationality"`
	Status             string     `json:"status" db:"status"`
	Version            int        `json:"version" db:"version"`
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at" db:"updated_at"`
}

// ApprovedIntake models approved seat capacity per institution/program/year
type ApprovedIntake struct {
	ID                   uuid.UUID `json:"id" db:"id"`
	InstitutionID        string    `json:"institution_id" db:"institution_id"`
	ProgramID            string    `json:"program_id" db:"program_id"`
	AcademicYear         string    `json:"academic_year" db:"academic_year"`
	RegularLimit         int       `json:"regular_limit" db:"regular_limit"`
	ApplicableAllowances int       `json:"applicable_allowances" db:"applicable_allowances"`
	PolicyVersion        string    `json:"policy_version" db:"policy_version"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time `json:"updated_at" db:"updated_at"`
}

// UploadJob tracks a CSV or API upload batch
type UploadJob struct {
	ID             uuid.UUID `json:"id" db:"id"`
	InstitutionID  string    `json:"institution_id" db:"institution_id"`
	CreatedBy      *uuid.UUID `json:"created_by,omitempty" db:"created_by"`
	Filename       string    `json:"filename" db:"filename"`
	TotalRows      int       `json:"total_rows" db:"total_rows"`
	ProcessedRows  int       `json:"processed_rows" db:"processed_rows"`
	AcceptedRows   int       `json:"accepted_rows" db:"accepted_rows"`
	FlaggedRows    int       `json:"flagged_rows" db:"flagged_rows"`
	RejectedRows   int       `json:"rejected_rows" db:"rejected_rows"`
	Status         string    `json:"status" db:"status"`
	IdempotencyKey *string   `json:"idempotency_key,omitempty" db:"idempotency_key"`
	ErrorSummary   *string   `json:"error_summary,omitempty" db:"error_summary"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// UploadRow preserves the exact submitted row payload and its final single outcome
type UploadRow struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	JobID            uuid.UUID  `json:"job_id" db:"job_id"`
	InstitutionID    string     `json:"institution_id" db:"institution_id"`
	RowNumber        int        `json:"row_number" db:"row_number"`
	RawPayload       []byte     `json:"raw_payload" db:"raw_payload"`
	NormalizedFields []byte     `json:"normalized_fields,omitempty" db:"normalized_fields"`
	ParseErrors      []byte     `json:"parse_errors,omitempty" db:"parse_errors"`
	Outcome          string     `json:"outcome" db:"outcome"`
	EnrolmentID      *uuid.UUID `json:"enrolment_id,omitempty" db:"enrolment_id"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

// ValidationFlag records rule validation review leads
type ValidationFlag struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	UploadRowID   *uuid.UUID `json:"upload_row_id,omitempty" db:"upload_row_id"`
	EnrolmentID   *uuid.UUID `json:"enrolment_id,omitempty" db:"enrolment_id"`
	InstitutionID string     `json:"institution_id" db:"institution_id"`
	RuleCode      string     `json:"rule_code" db:"rule_code"`
	Severity      string     `json:"severity" db:"severity"`
	Reason        string     `json:"reason" db:"reason"`
	Evidence      []byte     `json:"evidence,omitempty" db:"evidence"`
	PolicyVersion string     `json:"policy_version" db:"policy_version"`
	ReviewStatus  string     `json:"review_status" db:"review_status"`
	Reviewer      *uuid.UUID `json:"reviewer,omitempty" db:"reviewer"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty" db:"resolved_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}

// Consent models student permission for verifier access
type Consent struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	StudentAPAARID  string     `json:"student_apaar_id" db:"student_apaar_id"`
	VerifierID      uuid.UUID  `json:"verifier_id" db:"verifier_id"`
	EnrolmentID     *uuid.UUID `json:"enrolment_id,omitempty" db:"enrolment_id"`
	Purpose         string     `json:"purpose" db:"purpose"`
	PermittedFields []string   `json:"permitted_fields" db:"permitted_fields"`
	Status          string     `json:"status" db:"status"`
	Version         int        `json:"version" db:"version"`
	GrantedAt       time.Time  `json:"granted_at" db:"granted_at"`
	ExpiresAt       time.Time  `json:"expires_at" db:"expires_at"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}

// Credential holds Ed25519 signed verifiable payload with opaque unguessable QR ID
type Credential struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	OpaqueID         string     `json:"opaque_id" db:"opaque_id"`
	EnrolmentID      uuid.UUID  `json:"enrolment_id" db:"enrolment_id"`
	EnrolmentVersion int        `json:"enrolment_version" db:"enrolment_version"`
	CanonicalPayload []byte     `json:"canonical_payload" db:"canonical_payload"`
	Signature        []byte     `json:"signature" db:"signature"`
	KeyID            string     `json:"key_id" db:"key_id"`
	IssuedAt         time.Time  `json:"issued_at" db:"issued_at"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	SupersedesID     *uuid.UUID `json:"supersedes_id,omitempty" db:"supersedes_id"`
}

// AuditEvent represents a tamper-evident hash-chained audit record
type AuditEvent struct {
	ID            int64      `json:"id" db:"id"`
	ActorID       *uuid.UUID `json:"actor_id,omitempty" db:"actor_id"`
	ActorRole     string     `json:"actor_role" db:"actor_role"`
	Action        string     `json:"action" db:"action"`
	TargetType    string     `json:"target_type" db:"target_type"`
	TargetID      string     `json:"target_id" db:"target_id"`
	InstitutionID *string    `json:"institution_id,omitempty" db:"institution_id"`
	Details       []byte     `json:"details,omitempty" db:"details"`
	PrevHash      string     `json:"prev_hash" db:"prev_hash"`
	EventHash     string     `json:"event_hash" db:"event_hash"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}

// RollupEnrolment maintains pre-aggregated intake vs enrolled metrics
type RollupEnrolment struct {
	InstitutionID   string    `json:"institution_id" db:"institution_id"`
	ProgramID       string    `json:"program_id" db:"program_id"`
	AcademicYear    string    `json:"academic_year" db:"academic_year"`
	TotalEnrolled   int       `json:"total_enrolled" db:"total_enrolled"`
	Supernumerary   int       `json:"supernumerary" db:"supernumerary"`
	NRI             int       `json:"nri" db:"nri"`
	OCI             int       `json:"oci" db:"oci"`
	ForeignNational int       `json:"foreign_national" db:"foreign_national"`
	FlaggedCount    int       `json:"flagged_count" db:"flagged_count"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// APIKey represents a hashed API token
type APIKey struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	InstitutionID *string    `json:"institution_id,omitempty" db:"institution_id"`
	VerifierID    *uuid.UUID `json:"verifier_id,omitempty" db:"verifier_id"`
	KeyHash       string     `json:"-" db:"key_hash"`
	KeyPrefix     string     `json:"key_prefix" db:"key_prefix"`
	Scopes        []string   `json:"scopes" db:"scopes"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}

// WebhookDelivery records signed webhook dispatch state
type WebhookDelivery struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	EventID       uuid.UUID  `json:"event_id" db:"event_id"`
	EventType     string     `json:"event_type" db:"event_type"`
	InstitutionID *string    `json:"institution_id,omitempty" db:"institution_id"`
	FlagID        *uuid.UUID `json:"flag_id,omitempty" db:"flag_id"`
	RuleCode      string     `json:"rule_code" db:"rule_code"`
	Severity      string     `json:"severity" db:"severity"`
	Payload       []byte     `json:"payload" db:"payload"`
	Signature     string     `json:"signature" db:"signature"`
	Attempts      int        `json:"attempts" db:"attempts"`
	Status        string     `json:"status" db:"status"` // delivered, failed, retrying
	LastError     *string    `json:"last_error,omitempty" db:"last_error"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty" db:"delivered_at"`
}


