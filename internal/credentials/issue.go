package credentials

import (
	"context"
	"fmt"
	"time"

	"apaar-verify/internal/contracts"
	"apaar-verify/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// IssueCredential issues an Ed25519-signed verifiable credential for an institution-attested enrolment.
// Non-negotiable Rule #8: Credential issuance is restricted to authorized, attested enrolments.
func IssueCredential(
	ctx context.Context,
	tx pgx.Tx,
	signer contracts.Signer,
	enrolmentID uuid.UUID,
) (*models.Credential, error) {
	// 1. Verify enrolment exists, is ATTESTED, and version matches attested_version
	query := `
		SELECT status, version, COALESCE(attested_version, 0)
		FROM enrolments
		WHERE id = $1
	`
	var status string
	var version, attestedVersion int
	err := tx.QueryRow(ctx, query, enrolmentID).Scan(&status, &version, &attestedVersion)
	if err != nil {
		return nil, err
	}

	if status != "attested" {
		return nil, ErrNotAttested
	}

	if version != attestedVersion {
		return nil, fmt.Errorf("%w: current version %d != attested %d", ErrVersionMismatch, version, attestedVersion)
	}

	// 2. Check if a prior credential exists to supersede
	var priorCredID *uuid.UUID
	var tempID uuid.UUID
	err = tx.QueryRow(ctx, "SELECT id FROM credentials WHERE enrolment_id = $1 AND revoked_at IS NULL ORDER BY issued_at DESC LIMIT 1", enrolmentID).Scan(&tempID)
	if err == nil {
		priorCredID = &tempID
	}

	// 3. Generate unguessable opaque ID (>= 128 bits)
	opaqueID, err := GenerateOpaqueID()
	if err != nil {
		return nil, err
	}

	// 4. Construct deterministic versioned canonical payload
	credID := uuid.New()
	issuedAt := time.Now().UTC()
	canonicalBytes, err := BuildCanonicalPayload(signer.KeyID(), credID, enrolmentID, version, issuedAt, nil)
	if err != nil {
		return nil, err
	}

	// 5. Sign exact canonical bytes
	sig, keyID, err := signer.Sign(canonicalBytes)
	if err != nil {
		return nil, err
	}

	// 6. Store exact signed bytes in credentials table
	insertQuery := `
		INSERT INTO credentials (
			id, opaque_id, enrolment_id, enrolment_version, canonical_payload, signature, key_id, issued_at, supersedes_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err = tx.Exec(ctx, insertQuery, credID, opaqueID, enrolmentID, version, canonicalBytes, sig, keyID, issuedAt, priorCredID)
	if err != nil {
		return nil, err
	}

	return &models.Credential{
		ID:               credID,
		OpaqueID:         opaqueID,
		EnrolmentID:      enrolmentID,
		EnrolmentVersion: version,
		CanonicalPayload: canonicalBytes,
		Signature:        sig,
		KeyID:            keyID,
		IssuedAt:         issuedAt,
		SupersedesID:     priorCredID,
	}, nil
}
