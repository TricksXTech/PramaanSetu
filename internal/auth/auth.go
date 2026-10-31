package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"apaar-verify/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	SessionCookieName = "apaar_session"
	SessionDuration   = 24 * time.Hour
)

var (
	ErrSessionExpired  = errors.New("session has expired")
	ErrSessionNotFound = errors.New("session not found")
)

// HashToken computes SHA-256 hash for secure database storage
func HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// Service handles authentication and session lifecycle
type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// Authenticate verifies email and password against stored bcrypt hash
func (s *Service) Authenticate(ctx context.Context, email, password string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, role, institution_id, apaar_id, status, created_at, updated_at
		FROM users
		WHERE email = $1 AND status = 'active'
	`
	var u models.User
	err := s.db.QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.InstitutionID,
		&u.APAARID,
		&u.Status,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !CheckPasswordHash(password, u.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	return &u, nil
}

// CreateSession generates a secure random token, hashes it for DB storage, and returns the raw token
func (s *Service) CreateSession(ctx context.Context, user *models.User) (string, error) {
	rawToken, err := GenerateRandomToken(32)
	if err != nil {
		return "", err
	}

	tokenHash := HashToken(rawToken)
	expiresAt := time.Now().Add(SessionDuration)

	query := `
		INSERT INTO sessions (id, user_id, role, institution_id, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = s.db.Exec(ctx, query, tokenHash, user.ID, user.Role, user.InstitutionID, expiresAt)
	if err != nil {
		return "", err
	}

	return rawToken, nil
}

// SetSessionCookie sets a secure, HttpOnly session cookie
func SetSessionCookie(c *gin.Context, rawToken string) {
	c.SetSameSite(http.SameSiteLaxMode)
	// HttpOnly = true to prevent XSS access to session token
	c.SetCookie(SessionCookieName, rawToken, int(SessionDuration.Seconds()), "/", "", false, true)
}

// ClearSessionCookie clears the session cookie
func ClearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, "", -1, "/", "", false, true)
}

// ValidateSession verifies session token hash and returns the user and tenant details
func (s *Service) ValidateSession(ctx context.Context, rawToken string) (*models.User, *models.Session, error) {
	tokenHash := HashToken(rawToken)

	query := `
		SELECT s.id, s.user_id, s.role, s.institution_id, s.expires_at, s.created_at,
		       u.email, u.status, u.apaar_id
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.id = $1
	`
	var sess models.Session
	var u models.User

	err := s.db.QueryRow(ctx, query, tokenHash).Scan(
		&sess.ID,
		&sess.UserID,
		&sess.Role,
		&sess.InstitutionID,
		&sess.ExpiresAt,
		&sess.CreatedAt,
		&u.Email,
		&u.Status,
		&u.APAARID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrSessionNotFound
		}
		return nil, nil, err
	}

	if time.Now().After(sess.ExpiresAt) {
		// Clean up expired session
		_, _ = s.db.Exec(ctx, "DELETE FROM sessions WHERE id = $1", tokenHash)
		return nil, nil, ErrSessionExpired
	}

	u.ID = sess.UserID
	u.Role = sess.Role
	u.InstitutionID = sess.InstitutionID

	return &u, &sess, nil
}

// RevokeSession deletes the active session
func (s *Service) RevokeSession(ctx context.Context, rawToken string) error {
	tokenHash := HashToken(rawToken)
	_, err := s.db.Exec(ctx, "DELETE FROM sessions WHERE id = $1", tokenHash)
	return err
}

