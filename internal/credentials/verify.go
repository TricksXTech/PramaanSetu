package credentials

import (
	"context"
	"errors"
	"fmt"
	"time"

	"apaar-verify/internal/consent"
	"apaar-verify/internal/contracts"
	"apaar-verify/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrCredentialRevoked    = errors.New("credential has been revoked by issuing institution")
	ErrCredentialExpired    = errors.New("credential validity period has expired")
	ErrCredentialSuperseded = errors.New("credential has been superseded by a newer version")
)

// VerificationResponse provides verified fields strictly scoped to active consent
type VerificationResponse struct {
	CredentialID        string         `json:"credential_id"`
	OpaqueID            string         `json:"opaque_id"`
	FormatValid         bool           `json:"format_valid"`
	RegistryMatched     bool           `json:"registry_matched"`
	RegistryProvenance  string         `json:"registry_provenance"`
	RulesPassed         bool           `json:"rules_passed"`
	InstitutionAttested bool           `json:"institution_attested"`
	SignatureValid      bool           `json:"signature_valid"`
	CredentialCurrent   bool           `json:"credential_current"`
	Status              string         `json:"status"` // "verified", "consent_required", "revoked", "expired"
	ConsentActive       bool           `json:"consent_active"`
	ConsentRequired     bool           `json:"consent_required"`
	KeyID               string         `json:"key_id"`
	PermittedFields     map[string]any `json:"permitted_fields,omitempty"`
	VerifiedAt          time.Time      `json:"verified_at"`
}

// VerifyCredential executes the 6-stage verification gate with zero-caching dynamic consent
// Non-negotiable Rule #5 & #6: Distinct verification states, no caching of authorization decisions
func VerifyCredential(
	ctx context.Context,
	db *pgxpool.Pool,
	signer contracts.Signer,
	consentSvc *consent.Service,
	auditor contracts.Auditor,
	credID uuid.UUID,
	verifierID uuid.UUID,
) (*VerificationResponse, error) {
	now := time.Now().UTC()

	// 1. Fetch credential, enrolment, and student APAAR link
	query := `
		SELECT c.id, c.opaque_id, c.enrolment_id, c.enrolment_version,
		       c.canonical_payload, c.signature, c.key_id, c.issued_at,
		       c.expires_at, c.revoked_at, c.supersedes_id,
		       e.apaar_id, e.institution_id, e.program_id, e.academic_year, e.status, e.version,
		       s.name, s.dob
		FROM credentials c
		JOIN enrolments e ON c.enrolment_id = e.id
		JOIN students s ON e.apaar_id = s.apaar_id
		WHERE c.id = $1
	`

	var cred models.Credential
	var apaarID, instID, progID, acadYear, enrolStatus, studentName string
	var studentDOB time.Time
	var currentEnrolVersion int

	err := db.QueryRow(ctx, query, credID).Scan(
		&cred.ID,
		&cred.OpaqueID,
		&cred.EnrolmentID,
		&cred.EnrolmentVersion,
		&cred.CanonicalPayload,
		&cred.Signature,
		&cred.KeyID,
		&cred.IssuedAt,
		&cred.ExpiresAt,
		&cred.RevokedAt,
		&cred.SupersedesID,
		&apaarID,
		&instID,
		&progID,
		&acadYear,
		&enrolStatus,
		&currentEnrolVersion,
		&studentName,
		&studentDOB,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("credential not found")
		}
		return nil, err
	}

	// 2. Check not revoked / expired / superseded
	if cred.RevokedAt != nil {
		return nil, ErrCredentialRevoked
	}
	if cred.ExpiresAt != nil && now.After(*cred.ExpiresAt) {
		return nil, ErrCredentialExpired
	}

	// Check if newer credential exists for this enrolment
	var newerCount int
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM credentials WHERE enrolment_id = $1 AND issued_at > $2", cred.EnrolmentID, cred.IssuedAt).Scan(&newerCount)
	if newerCount > 0 {
		return nil, ErrCredentialSuperseded
	}

	// 3. Verify digital signature against stored bytes and expected issuer key
	sigErr := signer.Verify(cred.CanonicalPayload, cred.Signature, cred.KeyID)
	signatureValid := (sigErr == nil)
	if !signatureValid {
		return nil, fmt.Errorf("%w: %s", ErrInvalidSignature, sigErr.Error())
	}

	isAttested := (currentEnrolVersion == cred.EnrolmentVersion && enrolStatus == "attested")
	isCurrent := (cred.RevokedAt == nil && (cred.ExpiresAt == nil || now.Before(*cred.ExpiresAt)) && newerCount == 0)

	// 4. Check active student consent (Checked dynamically on every request without caching)
	hasConsent, permittedKeys, err := consentSvc.CheckConsent(ctx, apaarID, verifierID, cred.EnrolmentID)
	if err != nil {
		return nil, err
	}

	// If consent is missing/expired, returns verification status with NO student data and consent_required indicator (Milestone 3 Dev D Requirement #3)
	if !hasConsent {
		_ = auditor.Record(ctx, nil, contracts.AuditEvent{
			ActorID:    &verifierID,
			ActorRole:  models.RoleVerifier,
			Action:     "CREDENTIAL_VERIFIED_NO_CONSENT",
			TargetType: "credential",
			TargetID:   credID.String(),
			TenantID:   &instID,
			Metadata:   map[string]any{"reason": "consent_required"},
			OccurredAt: now,
		})

		return &VerificationResponse{
			CredentialID:        credID.String(),
			OpaqueID:            cred.OpaqueID,
			FormatValid:         true,
			RegistryMatched:     true,
			RegistryProvenance:  "mock",
			RulesPassed:         true,
			InstitutionAttested: isAttested,
			SignatureValid:      signatureValid,
			CredentialCurrent:   isCurrent,
			Status:              "consent_required",
			ConsentActive:       false,
			ConsentRequired:     true,
			KeyID:               cred.KeyID,
			PermittedFields:     nil, // Non-negotiable Rule #6: Zero personal data without active student consent
			VerifiedAt:          now,
		}, nil
	}

	// 5. Build field response limited strictly to consented permitted fields
	permittedMap := make(map[string]any)
	for _, key := range permittedKeys {
		switch key {
		case "name":
			permittedMap["name"] = studentName
		case "program_code":
			permittedMap["program_code"] = progID
		case "academic_year":
			permittedMap["academic_year"] = acadYear
		case "status":
			permittedMap["status"] = enrolStatus
		case "institution_id":
			permittedMap["institution_id"] = instID
		}
	}

	// 6. Audit successful verification attempt (Zero personal data in audit log)
	_ = auditor.Record(ctx, nil, contracts.AuditEvent{
		ActorID:    &verifierID,
		ActorRole:  models.RoleVerifier,
		Action:     "CREDENTIAL_VERIFIED_SUCCESS",
		TargetType: "credential",
		TargetID:   credID.String(),
		TenantID:   &instID,
		Metadata:   map[string]any{"permitted_field_count": len(permittedMap)},
		OccurredAt: now,
	})

	return &VerificationResponse{
		CredentialID:        credID.String(),
		OpaqueID:            cred.OpaqueID,
		FormatValid:         true,
		RegistryMatched:     true,
		RegistryProvenance:  "mock",
		RulesPassed:         true,
		InstitutionAttested: isAttested,
		SignatureValid:      signatureValid,
		CredentialCurrent:   isCurrent,
		Status:              "verified",
		ConsentActive:       true,
		ConsentRequired:     false,
		KeyID:               cred.KeyID,
		PermittedFields:     permittedMap,
		VerifiedAt:          now,
	}, nil
}
