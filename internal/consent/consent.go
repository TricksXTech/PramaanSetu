package consent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"apaar-verify/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrConsentDenied    = errors.New("consent denied: active student consent required")
	ErrNotStudentRecord = errors.New("security violation: cannot grant consent on another student's enrolment")
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// GrantConsent records dynamic verifier disclosure permissions
// Non-negotiable Rule #6: Dynamic student consent, checked on every request without caching
func (s *Service) GrantConsent(
	ctx context.Context,
	studentAPAAR string,
	verifierID uuid.UUID,
	enrolmentID *uuid.UUID,
	purpose string,
	permittedFields []string,
	expiresAt time.Time,
) (*models.Consent, error) {
	if s.db == nil {
		return nil, errors.New("database pool is nil")
	}
	// 1. If enrolmentID is specified, verify ownership
	if enrolmentID != nil {
		var count int
		err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM enrolments WHERE id = $1 AND apaar_id = $2", *enrolmentID, studentAPAAR).Scan(&count)
		if err != nil || count == 0 {
			return nil, ErrNotStudentRecord
		}
	}

	consentID := uuid.New()
	query := `
		INSERT INTO consents (
			id, student_apaar_id, verifier_id, enrolment_id, purpose, permitted_fields, status, version, granted_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'active', 1, CURRENT_TIMESTAMP, $7)
		RETURNING granted_at
	`
	var grantedAt time.Time
	err := s.db.QueryRow(ctx, query, consentID, studentAPAAR, verifierID, enrolmentID, purpose, permittedFields, expiresAt).Scan(&grantedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to grant consent: %w", err)
	}

	return &models.Consent{
		ID:              consentID,
		StudentAPAARID:  studentAPAAR,
		VerifierID:      verifierID,
		EnrolmentID:     enrolmentID,
		Purpose:         purpose,
		PermittedFields: permittedFields,
		Status:          models.ConsentStatusActive,
		Version:         1,
		GrantedAt:       grantedAt,
		ExpiresAt:       expiresAt,
	}, nil
}

// RevokeConsent revokes active consent immediately. Revocation stops future access.
func (s *Service) RevokeConsent(ctx context.Context, studentAPAAR string, consentID uuid.UUID) error {
	if s.db == nil {
		return errors.New("database pool is nil")
	}
	query := `
		UPDATE consents
		SET status = 'revoked', revoked_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND student_apaar_id = $2 AND status = 'active'
	`
	cmd, err := s.db.Exec(ctx, query, consentID, studentAPAAR)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("consent not found, already revoked, or unauthorized")
	}
	return nil
}

// CheckConsent strictly verifies whether active consent exists for verifier lookup
// Non-negotiable Rule #6: Check consent on every protected request with zero caching
func (s *Service) CheckConsent(
	ctx context.Context,
	studentAPAAR string,
	verifierID uuid.UUID,
	enrolmentID uuid.UUID,
) (bool, []string, error) {
	if s.db == nil {
		return false, nil, errors.New("database pool is nil")
	}
	query := `
		SELECT permitted_fields, expires_at
		FROM consents
		WHERE student_apaar_id = $1
		  AND verifier_id = $2
		  AND (enrolment_id IS NULL OR enrolment_id = $3)
		  AND status = 'active'
		  AND expires_at > CURRENT_TIMESTAMP
		ORDER BY granted_at DESC
		LIMIT 1
	`
	var permittedFields []string
	var expiresAt time.Time
	err := s.db.QueryRow(ctx, query, studentAPAAR, verifierID, enrolmentID).Scan(&permittedFields, &expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil, nil
		}
		return false, nil, err
	}

	return true, permittedFields, nil
}
