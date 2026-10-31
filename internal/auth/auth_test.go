package auth_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"apaar-verify/internal/auth"
	"github.com/gin-gonic/gin"
)

func TestPasswordHashing(t *testing.T) {
	password := "SecretP@ssw0rd123"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if hash == password {
		t.Fatalf("hash should not equal plain text")
	}

	if !auth.CheckPasswordHash(password, hash) {
		t.Fatalf("password check failed for correct password")
	}

	if auth.CheckPasswordHash("WrongPassword", hash) {
		t.Fatalf("password check succeeded for incorrect password")
	}
}

func TestCSRFMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(auth.CSRFMiddleware())

	r.GET("/form", func(c *gin.Context) {
		token, _ := c.Get(auth.CSRFContextKey)
		c.String(http.StatusOK, token.(string))
	})

	r.POST("/submit", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	// 1. GET request generates a CSRF cookie and context token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/form", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET, got %d", w.Code)
	}

	var csrfCookie *http.Cookie
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == auth.CSRFCookieName {
			csrfCookie = cookie
			break
		}
	}
	if csrfCookie == nil || csrfCookie.Value == "" {
		t.Fatalf("expected CSRF cookie to be set")
	}

	// 2. Mutating POST without CSRF token -> MUST return 403 Forbidden
	w2 := httptest.NewRecorder()
	postReq, _ := http.NewRequest(http.MethodPost, "/submit", nil)
	postReq.AddCookie(csrfCookie)
	r.ServeHTTP(w2, postReq)

	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden without CSRF token, got %d", w2.Code)
	}

	// 3. Mutating POST with valid CSRF token in header -> MUST SUCCEED (200 OK)
	w3 := httptest.NewRecorder()
	postReqValid, _ := http.NewRequest(http.MethodPost, "/submit", nil)
	postReqValid.AddCookie(csrfCookie)
	postReqValid.Header.Set(auth.CSRFHeaderName, csrfCookie.Value)
	r.ServeHTTP(w3, postReqValid)

	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with valid CSRF token, got %d", w3.Code)
	}

	// 4. Mutating POST with valid CSRF token in form body -> MUST SUCCEED (200 OK)
	w4 := httptest.NewRecorder()
	form := url.Values{}
	form.Set(auth.CSRFFormField, csrfCookie.Value)
	postReqForm, _ := http.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	postReqForm.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReqForm.AddCookie(csrfCookie)
	r.ServeHTTP(w4, postReqForm)

	if w4.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with valid CSRF form token, got %d", w4.Code)
	}
}

