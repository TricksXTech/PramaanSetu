package contracts

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CSVRow defines the normalized columns expected from institution uploads
type CSVRow struct {
	APAARID            string `json:"apaar_id"`
	StudentName        string `json:"student_name"`
	DOB                string `json:"dob"` // YYYY-MM-DD
	Gender             string `json:"gender,omitempty"`
	GuardianName       string `json:"guardian_name,omitempty"`
	ProgramCode        string `json:"program_code"`
	AcademicYear       string `json:"academic_year"`
	AdmissionReference string `json:"admission_reference"`
	Category           string `json:"category"`
	AdmissionType      string `json:"admission_type"`
	IsSupernumerary    bool   `json:"is_supernumerary"`
	IsNRI              bool   `json:"is_nri"`
	OCIStatus          bool   `json:"oci_status"`
	IsForeignNational  bool   `json:"is_foreign_national"`
	Nationality        string `json:"nationality"`
}

// RowOutcome represents the exact processing outcome of an uploaded row
type RowOutcome string

const (
	RowOutcomeAccepted        RowOutcome = "accepted"
	RowOutcomeFlaggedHeld     RowOutcome = "flagged_held"
	RowOutcomeRejectedParse   RowOutcome = "rejected_parse"
	RowOutcomeRejectedInvalid RowOutcome = "rejected_invalid"
	RowOutcomeNoOpIdentical   RowOutcome = "no_op_identical"
	RowOutcomeConflictStale   RowOutcome = "conflict_stale"
	RowOutcomePendingRegistry RowOutcome = "pending_registry"
)

// TargetType defines the target entity of a validation flag
type TargetType string

const (
	TargetUploadRow   TargetType = "upload_row_id"
	TargetEnrolment   TargetType = "enrolment_id"
	TargetInstitution TargetType = "institution_id"
)

// Flag models an explicit review lead or validation finding
type Flag struct {
	RuleCode      string         `json:"rule_code"`
	Severity      string         `json:"severity"` // "info", "warning", "error", "conflict"
	Reason        string         `json:"reason"`
	Evidence      map[string]any `json:"evidence"` // No raw personal payloads (Rule #7 & #8)
	TargetType    TargetType     `json:"target_type"`
	TargetID      string         `json:"target_id"`
	PolicyVersion string         `json:"policy_version"`
}

// ValidationMetadata provides contextual information for row validation
type ValidationMetadata struct {
	JobID         uuid.UUID
	InstitutionID string
	UploadRowID   uuid.UUID
	RowNumber     int
	SeenInFile    map[string]int // Tracks occurrences in current batch
	PolicyConfig  *PolicyConfig
}

// Validator validates a single normalized row inside an active database transaction
type Validator interface {
	Validate(ctx context.Context, tx pgx.Tx, row *CSVRow, meta *ValidationMetadata) ([]Flag, RowOutcome, error)
}

// RegistryStatus enumerates possible responses from APAAR registry lookup
type RegistryStatus string

const (
	RegistryStatusMatched          RegistryStatus = "matched"
	RegistryStatusNotFound         RegistryStatus = "not_found"
	RegistryStatusUnavailable      RegistryStatus = "unavailable"
	RegistryStatusIdentityMismatch RegistryStatus = "identity_mismatch"
)

// RegistryResult captures the response from the APAAR registry adapter
type RegistryResult struct {
	Status     RegistryStatus `json:"status"`
	Provenance string         `json:"provenance"` // Always "mock" in prototype
	Details    map[string]any `json:"details,omitempty"`
}

// RegistryAdapter provides APAAR registry lookup functionality
type RegistryAdapter interface {
	Lookup(ctx context.Context, apaarID string) (RegistryResult, error)
}

// Signer handles digital signing and signature verification of verifiable credentials
type Signer interface {
	Sign(payload []byte) (sig []byte, keyID string, err error)
	Verify(payload, sig []byte, keyID string) error
	KeyID() string
}

// CanonicalCredentialPayload specifies the deterministic versioned payload
type CanonicalCredentialPayload struct {
	Version          string     `json:"version"` // "1.0"
	Issuer           string     `json:"issuer"`  // "AICTE"
	KeyID            string     `json:"key_id"`
	CredentialID     uuid.UUID  `json:"credential_id"`
	EnrolmentID      uuid.UUID  `json:"enrolment_id"`
	EnrolmentVersion int        `json:"enrolment_version"`
	IssuedAt         time.Time  `json:"issued_at"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
}

// ================= MILESTONE 3 EXTENSIONS (Step 0) =================

// PolicyConfig defines versioned regulatory rules for validation and intake
type PolicyConfig struct {
	PolicyVersion           string              `json:"policy_version"`
	AllowedCategories       []string            `json:"allowed_categories"`
	AllowedAdmissionTypes   []string            `json:"allowed_admission_types"`
	MinAgeYears             int                 `json:"min_age_years"`
	MaxAgeYears             int                 `json:"max_age_years"`
	SupernumeraryAllowances map[string]int      `json:"supernumerary_allowances"` // Percentage or fixed seats
	RequiredCategoryFields  map[string][]string `json:"required_category_fields"`
}

// IntakeResult encapsulates the result of evaluating an admission row against approved capacity
type IntakeResult struct {
	RegularLimit          int  `json:"regular_limit"`
	Allowances            int  `json:"allowances"`
	AcceptedRegular       int  `json:"accepted_regular"`
	AcceptedSupernumerary int  `json:"accepted_supernumerary"`
	OverflowRegular       bool `json:"overflow_regular"`
	OverflowSupernumerary bool `json:"overflow_supernumerary"`
}

// AuditEvent represents a tamper-evident audit record payload (NO personal data)
type AuditEvent struct {
	ActorID    *uuid.UUID     `json:"actor_id,omitempty"`
	ActorRole  string         `json:"actor_role"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	TenantID   *string        `json:"tenant_id,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"` // Explicitly devoid of PII
	OccurredAt time.Time      `json:"occurred_at"`
}

// Auditor defines the transactional recording interface for tamper-evident logging
type Auditor interface {
	Record(ctx context.Context, tx pgx.Tx, event AuditEvent) error
}

// RollupKey identifies the composite primary key for enrolment aggregations
type RollupKey struct {
	InstitutionID string `json:"institution_id"`
	ProgramID     string `json:"program_id"`
	AcademicYear  string `json:"academic_year"`
}

// RollupCounts tracks aggregated metrics per program and year
type RollupCounts struct {
	Accepted        int `json:"accepted"`
	Supernumerary   int `json:"supernumerary"`
	NRI             int `json:"nri"`
	OCI             int `json:"oci"`
	FN              int `json:"fn"`
	FlaggedOpen     int `json:"flagged_open"`
	FlaggedResolved int `json:"flagged_resolved"`
	Held            int `json:"held"`
}

// ================= MILESTONE 4 EXTENSIONS (Step 0) =================

// MatchCandidate models a potential fuzzy identity match candidate
type MatchCandidate struct {
	SourceRowOrEnrolmentRef string         `json:"source_row_or_enrolment_ref"`
	CandidateEnrolmentRef   string         `json:"candidate_enrolment_ref"`
	Score                   float64        `json:"score"`
	MatchedFields           []string       `json:"matched_fields"`
	BlockingKey             string         `json:"blocking_key"`
	Method                  string         `json:"method"`
	ThresholdsUsed          map[string]any `json:"thresholds_used"`
}

// MatchConfig provides bounded fuzzy matching thresholds and bounds (Rule #4)
type MatchConfig struct {
	MinScore            float64  `json:"min_score"`
	MaxCandidatesPerRow int      `json:"max_candidates_per_row"`
	QueryTimeoutMs      int      `json:"query_timeout_ms"`
	BlockingStrategies  []string `json:"blocking_strategies"`
	ConfigVersion       string   `json:"config_version"`
}

// DefaultMatchConfig returns standardized demo values clearly labelled
func DefaultMatchConfig() MatchConfig {
	return MatchConfig{
		MinScore:            0.65,
		MaxCandidatesPerRow: 5,
		QueryTimeoutMs:      250,
		BlockingStrategies:  []string{"dob_exact+name_trgm", "dob_year+name_soundex"},
		ConfigVersion:       "MATCH-CONFIG-DEMO-V1",
	}
}

// CrossInstitutionEvidence structures evidence shown to AICTE reviewers
// List views expose masked identifiers only. Never full APAAR ID, DOB, or guardian name.
type CrossInstitutionEvidence struct {
	PrimaryInstitutionID   string  `json:"primary_institution_id"`
	PrimaryInstitutionName string  `json:"primary_institution_name,omitempty"`
	PrimaryProgramCode     string  `json:"primary_program_code"`
	PrimaryAcademicYear    string  `json:"primary_academic_year"`
	OtherInstitutionID     string  `json:"other_institution_id"`
	OtherInstitutionName   string  `json:"other_institution_name,omitempty"`
	OtherProgramCode       string  `json:"other_program_code"`
	OtherAcademicYear      string  `json:"other_academic_year"`
	MaskedAPAAR            string  `json:"masked_apaar"`
	MaskedDOB              string  `json:"masked_dob"`
	MaskedGuardianName     string  `json:"masked_guardian_name,omitempty"`
	CandidateScore         float64 `json:"candidate_score,omitempty"`
	BlockingMethod         string  `json:"blocking_method,omitempty"`
}

// ReviewDecision enumerates allowed verdicts in the AICTE review workflow
type ReviewDecision string

const (
	DecisionConfirmedSamePerson      ReviewDecision = "confirmed_same_person"
	DecisionConfirmedDifferentPeople ReviewDecision = "confirmed_different_people"
	DecisionValidConcurrentStudy    ReviewDecision = "valid_concurrent_study"
	DecisionNeedsMoreInformation     ReviewDecision = "needs_more_information"
	DecisionDismissed                ReviewDecision = "dismissed"
)

// ReviewAction records a single audited review workflow action
type ReviewAction struct {
	ReviewerID uuid.UUID      `json:"reviewer_id"`
	Decision   ReviewDecision `json:"decision"`
	Note       string         `json:"note"`
	DecidedAt  time.Time      `json:"decided_at"`
}

// AnomalyStatus enumerates institution-level enrolment volume states
type AnomalyStatus string

const (
	AnomalyStatusOK                  AnomalyStatus = "ok"
	AnomalyStatusWarning             AnomalyStatus = "warning"
	AnomalyStatusInsufficientHistory AnomalyStatus = "insufficient_history"
)

// AnomalyResult records statistical evaluation of institution enrolment volume (Rule V9)
type AnomalyResult struct {
	InstitutionID  string        `json:"institution_id"`
	Metric         string        `json:"metric"`
	Observed       float64       `json:"observed"`
	BaselineMean   float64       `json:"baseline_mean"`
	BaselineStdDev float64       `json:"baseline_stddev"`
	YearsOfHistory int           `json:"years_of_history"`
	Status         AnomalyStatus `json:"status"`
}

// MaskAPAAR masks an APAAR ID for reviewer list views (e.g. "1234********")
func MaskAPAAR(apaar string) string {
	if len(apaar) < 6 {
		return "******"
	}
	return apaar[:4] + "****" + apaar[len(apaar)-4:]
}

// MaskDOB masks a date of birth string (e.g. "2004-**-**")
func MaskDOB(dob string) string {
	if len(dob) >= 4 {
		return dob[:4] + "-**-**"
	}
	return "****-**-**"
}

// MaskName masks a student or guardian name (e.g. "R**** S****")
func MaskName(name string) string {
	parts := splitWhitespace(name)
	if len(parts) == 0 {
		return "****"
	}
	var masked []string
	for _, p := range parts {
		if len(p) <= 1 {
			masked = append(masked, "*")
		} else {
			masked = append(masked, string(p[0])+"***")
		}
	}
	return joinStrings(masked, " ")
}

func splitWhitespace(s string) []string {
	var words []string
	cur := ""
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			if len(cur) > 0 {
				words = append(words, cur)
				cur = ""
			}
		} else {
			cur += string(r)
		}
	}
	if len(cur) > 0 {
		words = append(words, cur)
	}
	return words
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	res := parts[0]
	for i := 1; i < len(parts); i++ {
		res += sep + parts[i]
	}
	return res
}

// ================= MILESTONE 5 EXTENSIONS (Step 0) =================

// APIKeyScope enumerates allowed permissions for API keys
type APIKeyScope string

const (
	ScopeUploadsWrite      APIKeyScope = "uploads:write"
	ScopeUploadsRead       APIKeyScope = "uploads:read"
	ScopeEnrolmentsWrite   APIKeyScope = "enrolments:write"
	ScopeCredentialsVerify APIKeyScope = "credentials:verify"
)

// ReportType identifies the analytical aggregation report
type ReportType string

const (
	ReportIntakeVsEnrolment ReportType = "intake_vs_enrolment"
	ReportCategoryBreakdown ReportType = "category_breakdown"
	ReportFlaggedSummary    ReportType = "flagged_summary"
)

// ReportSpec specifies parameters for generated reports (aggregates only, no PII)
type ReportSpec struct {
	ReportType    ReportType `json:"report_type"`
	Scope         string     `json:"scope"` // "national" or institution_id
	AcademicYear  string     `json:"academic_year"`
	GeneratedBy   string     `json:"generated_by"`
	GeneratedAt   time.Time  `json:"generated_at"`
	PolicyVersion string     `json:"policy_version"`
}

// WebhookEvent models a signed regulatory flag event payload (identifiers only, zero PII)
type WebhookEvent struct {
	EventID       uuid.UUID `json:"event_id"`
	EventType     string    `json:"event_type"` // "flag.created"
	OccurredAt    time.Time `json:"occurred_at"`
	InstitutionID string    `json:"institution_id"`
	FlagID        uuid.UUID `json:"flag_id"`
	RuleCode      string    `json:"rule_code"`
	Severity      string    `json:"severity"`
}

// CertificateView defines the exact fields shown on the printable credential certificate
type CertificateView struct {
	CredentialID       uuid.UUID  `json:"credential_id"`
	OpaqueID           string     `json:"opaque_id"`
	StudentName        string     `json:"student_name"`
	AdmissionReference string     `json:"admission_reference"`
	APAARID            string     `json:"apaar_id"`
	ProgramCode        string     `json:"program_code"`
	ProgramName        string     `json:"program_name"`
	AcademicYear       string     `json:"academic_year"`
	InstitutionID      string     `json:"institution_id"`
	InstitutionName    string     `json:"institution_name"`
	CredentialStatus   string     `json:"credential_status"` // "active", "revoked", "expired", "superseded"
	RevocationReason   *string    `json:"revocation_reason,omitempty"`
	IssuedAt           time.Time  `json:"issued_at"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	VerifyURL          string     `json:"verify_url"`
	QRCodeBase64       string     `json:"qr_code_base64,omitempty"`
}
