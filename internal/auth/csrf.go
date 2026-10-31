package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	CSRFCookieName = "apaar_csrf"
	CSRFContextKey = "csrf_token"
	CSRFFormField  = "_csrf"
	CSRFHeaderName = "X-CSRF-Token"
)

// GenerateRandomToken generates a hex-encoded crypto-safe random string
func GenerateRandomToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CSRFMiddleware handles double-submit cookie pattern / session CSRF protection
func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// API and webhook endpoints bypass browser CSRF
		if strings.HasPrefix(c.Request.URL.Path, "/api/v1/") || strings.HasPrefix(c.Request.URL.Path, "/demo/webhook-") {
			c.Next()
			return
		}

		token, err := c.Cookie(CSRFCookieName)
		if err != nil || token == "" {
			token, err = GenerateRandomToken(32)
			if err == nil {
				c.SetSameSite(http.SameSiteLaxMode)
				c.SetCookie(CSRFCookieName, token, 86400, "/", "", false, false) // Accessible by JS/HTMX if needed, or matched against form
			}
		}

		// Store in context for html/template rendering
		c.Set(CSRFContextKey, token)

		// For mutating requests, enforce token check
		method := c.Request.Method
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete {
			// Extract sent token from header or form field
			sentToken := c.GetHeader(CSRFHeaderName)
			if sentToken == "" {
				sentToken = c.PostForm(CSRFFormField)
			}
			if sentToken == "" {
				sentToken = c.PostForm(CSRFContextKey)
			}

			if sentToken == "" || subtle.ConstantTimeCompare([]byte(sentToken), []byte(token)) != 1 {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error":   "Forbidden",
					"message": "Invalid or missing CSRF token",
				})
				return
			}
		}

		c.Next()
	}
}

