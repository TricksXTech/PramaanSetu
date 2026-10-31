package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"apaar-verify/internal/contracts"
	"apaar-verify/internal/models"
	"apaar-verify/internal/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidAPIKey = errors.New("invalid or expired API key")
	ErrForbiddenScope = errors.New("insufficient API key scope for requested operation")
)

type APIKeyService struct {
	db *pgxpool.Pool
}

func NewAPIKeyService(db *pgxpool.Pool) *APIKeyService {
	return &APIKeyService{db: db}
}

// CreateKey generates a high-entropy random API key and persists its SHA-256 hash
// Non-negotiable Rule #8: Secrets never in repo, stored as hashes only, raw secret returned once
func (s *APIKeyService) CreateKey(
	ctx context.Context,
	institutionID *string,
	verifierID *uuid.UUID,
	scopes []string,
	expiryDays int,
) (string, *models.APIKey, error) {
	if institutionID == nil && verifierID == nil {
		return "", nil, errors.New("API key must belong to either an institution or a verifier")
	}
	if institutionID != nil && verifierID != nil {
		return "", nil, errors.New("API key cannot belong to both an institution and a verifier")
	}
	if len(scopes) == 0 {
		return "", nil, errors.New("at least one scope is required")
	}

	// Generate 24 cryptographically random bytes -> 48 hex chars
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	prefix := "ak_live_"
	rawKey := prefix + hex.EncodeToString(b)
	keyHash := HashToken(rawKey)
	keyPrefix := rawKey[:16]

	var expiresAt *time.Time
	if expiryDays > 0 {
		exp := time.Now().UTC().AddDate(0, 0, expiryDays)
		expiresAt = &exp
	}

	keyID := uuid.New()
	query := `
		INSERT INTO api_keys (id, institution_id, verifier_id, key_hash, key_prefix, scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at
	`
	var createdAt time.Time
	err := s.db.QueryRow(ctx, query, keyID, institutionID, verifierID, keyHash, keyPrefix, scopes, expiresAt).Scan(&createdAt)
	if err != nil {
		return "", nil, fmt.Errorf("failed to persist API key: %w", err)
	}

	return rawKey, &models.APIKey{
		ID:            keyID,
		InstitutionID: institutionID,
		VerifierID:    verifierID,
		KeyHash:       keyHash,
		KeyPrefix:     keyPrefix,
		Scopes:        scopes,
		ExpiresAt:     expiresAt,
		CreatedAt:     createdAt,
	}, nil
}

// ListKeys returns API keys for an institution or verifier (excluding secrets)
func (s *APIKeyService) ListKeys(ctx context.Context, institutionID *string, verifierID *uuid.UUID) ([]models.APIKey, error) {
	query := `
		SELECT id, institution_id, verifier_id, key_prefix, scopes, expires_at, revoked_at, last_used_at, created_at
		FROM api_keys
		WHERE ($1::varchar IS NULL OR institution_id = $1)
		  AND ($2::uuid IS NULL OR verifier_id = $2)
		ORDER BY created_at DESC
	`
	rows, err := s.db.Query(ctx, query, institutionID, verifierID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.InstitutionID, &k.VerifierID, &k.KeyPrefix, &k.Scopes, &k.ExpiresAt, &k.RevokedAt, &k.LastUsedAt, &k.CreatedAt); err == nil {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

// RevokeKey revokes an API key immediately
func (s *APIKeyService) RevokeKey(ctx context.Context, keyID uuid.UUID, institutionID *string) error {
	query := `
		UPDATE api_keys
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND ($2::varchar IS NULL OR institution_id = $2) AND revoked_at IS NULL
	`
	cmd, err := s.db.Exec(ctx, query, keyID, institutionID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("API key not found, already revoked, or unauthorized")
	}
	return nil
}

// AuthenticateAndAuthorizeKey validates the token, checks revocation/expiry, and enforces required scope
func (s *APIKeyService) AuthenticateAndAuthorizeKey(
	ctx context.Context,
	rawToken string,
	requiredScope contracts.APIKeyScope,
) (*tenant.Scope, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, ErrInvalidAPIKey
	}

	keyHash := HashToken(rawToken)
	query := `
		SELECT id, institution_id, verifier_id, scopes, expires_at, revoked_at
		FROM api_keys
		WHERE key_hash = $1
	`
	var keyID uuid.UUID
	var instID *string
	var verifierID *uuid.UUID
	var scopes []string
	var expiresAt, revokedAt *time.Time

	err := s.db.QueryRow(ctx, query, keyHash).Scan(
		&keyID, &instID, &verifierID, &scopes, &expiresAt, &revokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidAPIKey
		}
		return nil, err
	}

	// Check revocation and expiration
	now := time.Now().UTC()
	if revokedAt != nil {
		return nil, ErrInvalidAPIKey
	}
	if expiresAt != nil && now.After(*expiresAt) {
		return nil, ErrInvalidAPIKey
	}

	// Enforce required scope
	scopeFound := false
	for _, sc := range scopes {
		if sc == string(requiredScope) || sc == "*" {
			scopeFound = true
			break
		}
	}
	if !scopeFound {
		return nil, ErrForbiddenScope
	}

	// Update last_used_at asynchronously
	go func(id uuid.UUID) {
		_, _ = s.db.Exec(context.Background(), "UPDATE api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = $1", id)
	}(keyID)

	if instID != nil {
		return &tenant.Scope{
			Role:          models.RoleInstitutionAdmin,
			InstitutionID: *instID,
		}, nil
	}

	if verifierID != nil {
		return &tenant.Scope{
			UserID: *verifierID,
			Role:   models.RoleVerifier,
		}, nil
	}

	return nil, ErrInvalidAPIKey
}

