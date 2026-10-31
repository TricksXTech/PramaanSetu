package credentials

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrBlockingFlagsOpen = errors.New("cannot attest enrolment: open blocking validation flags (V1, V5, V6) must be resolved first")
	ErrVersionMismatch   = errors.New("attestation invalid: enrolment version has changed since attestation")
)

// AttestEnrolment records institutional attestation for an enrolment version.
// Milestone 3 Dev D Requirement #1: Block attestation while open blocking flags exist.
func AttestEnrolment(
	ctx context.Context,
	tx pgx.Tx,
	enrolmentID uuid.UUID,
	institutionID string,
	attesterID *uuid.UUID,
) (int, error) {
	// 1. Check for open blocking flags (V1, V5, V6)
	blockingQuery := `
		SELECT COUNT(*)
		FROM validation_flags
		WHERE enrolment_id = $1
		  AND rule_code IN ('V1', 'V5', 'V6')
		  AND review_status = 'pending'
	`
	var blockingCount int
	if err := tx.QueryRow(ctx, blockingQuery, enrolmentID).Scan(&blockingCount); err == nil && blockingCount > 0 {
		return 0, fmt.Errorf("%w: %d unresolved flags", ErrBlockingFlagsOpen, blockingCount)
	}

	// 2. Fetch current version and update enrolment status and attested_version
	updateQuery := `
		UPDATE enrolments
		SET status = 'attested',
		    attested_version = version,
		    attested_by = $1,
		    attested_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND institution_id = $3
		RETURNING version
	`
	var version int
	err := tx.QueryRow(ctx, updateQuery, attesterID, enrolmentID, institutionID).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to attest enrolment: %w", err)
	}

	return version, nil
}

// IsAttestationValid checks whether the enrolment version matches attested_version
func IsAttestationValid(version, attestedVersion int) bool {
	return version > 0 && version == attestedVersion
}

// RevokeCredential explicitly revokes an active credential
func RevokeCredential(ctx context.Context, tx pgx.Tx, credID uuid.UUID) error {
	return RevokeCredentialWithReason(ctx, tx, credID, "Administrative Revocation")
}

// RevokeCredentialWithReason revokes a credential and records the mandatory revocation reason
func RevokeCredentialWithReason(ctx context.Context, tx pgx.Tx, credID uuid.UUID, reason string) error {
	now := time.Now().UTC()
	query := `
		UPDATE credentials
		SET revoked_at = $1, revocation_reason = $2
		WHERE id = $3 AND revoked_at IS NULL
	`
	cmd, err := tx.Exec(ctx, query, now, reason, credID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("credential not found or already revoked")
	}
	return nil
}

