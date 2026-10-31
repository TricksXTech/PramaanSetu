package tenant

import (
	"errors"
	"fmt"

	"apaar-verify/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	ErrNoTenantInContext       = errors.New("no tenant scope found in context")
	ErrCrossTenantAccessDenied = errors.New("security violation: cross-tenant access denied")
	ErrStudentIdentityMismatch = errors.New("security violation: access denied to other student's record")
	ErrUnauthorizedRole        = errors.New("unauthorized role for tenant operation")
)

const ContextKey = "tenant_scope"

// Scope encapsulates the authenticated tenant identity for request execution
type Scope struct {
	UserID        uuid.UUID
	Role          string
	InstitutionID string // Non-empty for institution_admin
	APAARID       string // Non-empty for student
}

// FromContext extracts the TenantScope from Gin context
func FromContext(c *gin.Context) (*Scope, error) {
	val, exists := c.Get(ContextKey)
	if !exists {
		return nil, ErrNoTenantInContext
	}
	scope, ok := val.(*Scope)
	if !ok || scope == nil {
		return nil, ErrNoTenantInContext
	}
	return scope, nil
}

// SetInContext sets the TenantScope in Gin context
func SetInContext(c *gin.Context, scope *Scope) {
	c.Set(ContextKey, scope)
}

// EnforceInstitution verifies that the caller has authority over the given institutionID.
// Non-negotiable Rule #2: AICTE Admin can view all; Institution Admin can only access their own.
func (s *Scope) EnforceInstitution(targetInstitutionID string) error {
	if s == nil {
		return ErrNoTenantInContext
	}

	if s.Role == models.RoleAICTEAdmin {
		return nil // AICTE admin has national supervisory authority
	}

	if s.Role == models.RoleInstitutionAdmin {
		if s.InstitutionID == "" || s.InstitutionID != targetInstitutionID {
			return fmt.Errorf("%w: user from %q cannot access tenant %q", ErrCrossTenantAccessDenied, s.InstitutionID, targetInstitutionID)
		}
		return nil
	}

	return fmt.Errorf("%w: role %s has no institution-level access", ErrUnauthorizedRole, s.Role)
}

// EnforceStudent verifies that the student is only accessing their own record.
// Non-negotiable Rule #2: Student accounts can access only their own consent/credential records.
func (s *Scope) EnforceStudent(targetAPAARID string) error {
	if s == nil {
		return ErrNoTenantInContext
	}

	if s.Role == models.RoleAICTEAdmin {
		return nil // Regulators may view in audits
	}

	if s.Role == models.RoleStudent {
		if s.APAARID == "" || s.APAARID != targetAPAARID {
			return fmt.Errorf("%w: student %q cannot access data for %q", ErrStudentIdentityMismatch, s.APAARID, targetAPAARID)
		}
		return nil
	}

	return fmt.Errorf("%w: role %s cannot perform direct student actions", ErrUnauthorizedRole, s.Role)
}

// ScopedQuery ensures every SQL read/write enforces tenant ownership in the query itself.
// Returns the resolved tenant ID parameter to pass into the SQL query placeholder.
func (s *Scope) ScopedQuery(targetInstitutionID string) (string, error) {
	if err := s.EnforceInstitution(targetInstitutionID); err != nil {
		return "", err
	}
	return targetInstitutionID, nil
}

