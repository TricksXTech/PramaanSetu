package auth

import (
	"net/http"
	"strings"

	"apaar-verify/internal/models"
	"apaar-verify/internal/tenant"
	"github.com/gin-gonic/gin"
)

const (
	UserContextKey    = "current_user"
	SessionContextKey = "current_session"
)

// SessionMiddleware extracts and verifies the session cookie, setting User and TenantScope in context
func SessionMiddleware(authSvc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(SessionCookieName)
		if err == nil && token != "" {
			user, sess, err := authSvc.ValidateSession(c.Request.Context(), token)
			if err == nil && user != nil {
				c.Set(UserContextKey, user)
				c.Set(SessionContextKey, sess)

				var instID string
				if user.InstitutionID != nil {
					instID = *user.InstitutionID
				}
				var apaarID string
				if user.APAARID != nil {
					apaarID = *user.APAARID
				}

				scope := &tenant.Scope{
					UserID:        user.ID,
					Role:          user.Role,
					InstitutionID: instID,
					APAARID:       apaarID,
				}
				tenant.SetInContext(c, scope)
			}
		}

		c.Next()
	}
}

// RequireAuth ensures the request has an authenticated session
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := c.Get(UserContextKey)
		if !exists {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error":   "Unauthorized",
					"message": "Authentication required",
				})
				return
			}
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireRoles enforces role-based access control
func RequireRoles(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, err := tenant.FromContext(c)
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		allowed := false
		for _, r := range allowedRoles {
			if scope.Role == r {
				allowed = true
				break
			}
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "Forbidden",
				"message": "Access restricted: insufficient role permissions",
			})
			return
		}

		c.Next()
	}
}

// CurrentUser helper to retrieve user from Gin context
func CurrentUser(c *gin.Context) *models.User {
	val, exists := c.Get(UserContextKey)
	if !exists {
		return nil
	}
	u, ok := val.(*models.User)
	if !ok {
		return nil
	}
	return u
}

