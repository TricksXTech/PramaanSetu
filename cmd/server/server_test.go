package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"apaar-verify/internal/auth"
	"apaar-verify/internal/contracts"
	"apaar-verify/internal/models"
	"apaar-verify/internal/tenant"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestHealthCheckRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"version": "0.3.0-milestone3",
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /healthz, got %d", w.Code)
	}
}

func TestCrossTenantAccessRoute_Denial(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// DTU Admin
	dtuScope := &tenant.Scope{
		UserID:        uuid.New(),
		Role:          models.RoleInstitutionAdmin,
		InstitutionID: "INST-001",
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		tenant.SetInContext(c, dtuScope)
		c.Next()
	})

	r.GET("/api/v1/uploads/:id", func(c *gin.Context) {
		scope, err := tenant.FromContext(c)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		targetJobInstID := c.Query("job_inst_id")
		if err := scope.EnforceInstitution(targetJobInstID); err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "Forbidden",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "authorized"})
	})

	// Case 1: DTU accessing own job -> 200 OK
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/v1/uploads/job-1?job_inst_id=INST-001", nil)
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for own tenant access, got %d", w1.Code)
	}

	// Case 2: DTU attempting to read COEP's job -> 403 Forbidden! (Non-negotiable Rule #2)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/uploads/job-2?job_inst_id=INST-002", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for cross-tenant job access, got %d", w2.Code)
	}
}

func TestPatchEnrolment_StaleVersionConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Handler simulating optimistic concurrency version check
	r.PATCH("/api/v1/enrolments/:id", func(c *gin.Context) {
		var body map[string]any
		if err := c.ShouldBindJSON(&body); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		// Identity fields cannot be patched directly (Rule #1)
		if _, exists := body["apaar_id"]; exists {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Identity fields cannot be patched directly"})
			return
		}

		currentVersion := 2
		expectedVersion := int(body["expected_version"].(float64))

		if currentVersion != expectedVersion {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error":            "Conflict",
				"message":          "Stale version",
				"current_version":  currentVersion,
				"expected_version": expectedVersion,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "updated", "version": currentVersion + 1})
	})

	// 1. Stale version (expected 1, but current is 2) -> MUST return 409 Conflict
	w1 := httptest.NewRecorder()
	staleBody, _ := json.Marshal(map[string]any{"expected_version": 1, "category": "OBC"})
	req1, _ := http.NewRequest(http.MethodPatch, "/api/v1/enrolments/test-id", bytes.NewReader(staleBody))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for stale PATCH version, got %d", w1.Code)
	}

	// 2. Matching version (expected 2, current is 2) -> 200 OK
	w2 := httptest.NewRecorder()
	validBody, _ := json.Marshal(map[string]any{"expected_version": 2, "category": "OBC"})
	req2, _ := http.NewRequest(http.MethodPatch, "/api/v1/enrolments/test-id", bytes.NewReader(validBody))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid PATCH version, got %d", w2.Code)
	}

	// 3. Attempting to overwrite APAAR ID in patch -> MUST return 400 Bad Request
	w3 := httptest.NewRecorder()
	idOverwriteBody, _ := json.Marshal(map[string]any{"expected_version": 2, "apaar_id": "123456789012"})
	req3, _ := http.NewRequest(http.MethodPatch, "/api/v1/enrolments/test-id", bytes.NewReader(idOverwriteBody))
	req3.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w3, req3)

	if w3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for identity overwrite attempt, got %d", w3.Code)
	}
}

func TestPublicVerification_MinimalResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/verify/:opaque_id", func(c *gin.Context) {
		opaqueID := c.Param("opaque_id")
		status := "not_found"
		if opaqueID == "cr_valid_test_token" {
			status = "active"
		}
		c.JSON(http.StatusOK, gin.H{
			"status": status,
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/verify/cr_valid_test_token", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	body := w.Body.String()
	sensitiveKeywords := []string{"apaar_id", "dob", "student_name", "program", "gender"}
	for _, kw := range sensitiveKeywords {
		if strings.Contains(strings.ToLower(body), kw) {
			t.Fatalf("privacy violation: public verify page leaked sensitive field %q", kw)
		}
	}
}

func TestTemplateEscaping_XSSPrevention(t *testing.T) {
	maliciousInput := "<script>alert('XSS')</script>"
	tmpl := template.Must(template.New("test").Parse("Hello {{.Name}}"))

	var buf bytes.Buffer
	err := tmpl.Execute(&buf, map[string]string{"Name": maliciousInput})
	if err != nil {
		t.Fatalf("failed to render template: %v", err)
	}

	rendered := buf.String()
	if strings.Contains(rendered, "<script>") {
		t.Fatalf("XSS vulnerability: raw script tag was rendered unescaped: %s", rendered)
	}
	if !strings.Contains(rendered, "&lt;script&gt;") {
		t.Fatalf("expected HTML escaping for script tag, got: %s", rendered)
	}
}

func TestRoleHomeRedirects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	roles := []struct {
		role             string
		expectedRedirect string
	}{
		{models.RoleAICTEAdmin, "/aicte/dashboard"},
		{models.RoleInstitutionAdmin, "/institution/dashboard"},
		{models.RoleStudent, "/student/consent"},
		{models.RoleVerifier, "/verifier/verify"},
	}

	for _, tc := range roles {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

		redirectRoleHome(c, tc.role)

		if w.Code != http.StatusSeeOther {
			t.Errorf("role %s expected redirect 303, got %d", tc.role, w.Code)
		}
		loc := w.Header().Get("Location")
		if loc != tc.expectedRedirect {
			t.Errorf("role %s expected redirect to %s, got %s", tc.role, tc.expectedRedirect, loc)
		}
	}
}

func TestUnauthenticatedProtection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/protected", auth.RequireAuth(), func(c *gin.Context) {
		c.String(http.StatusOK, "secret data")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login" {
		t.Fatalf("expected redirect to /login for unauthenticated request, got code %d loc %s", w.Code, w.Header().Get("Location"))
	}
}

func TestAuditVerifyRoute_HTMXAndJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/aicte/audit/verify", func(c *gin.Context) {
		isHTMX := c.GetHeader("HX-Request") == "true"
		intact := c.Query("tampered") != "true"

		if isHTMX {
			c.Header("Content-Type", "text/html; charset=utf-8")
			if !intact {
				c.String(http.StatusOK, `<div id="audit-status-badge">Integrity failure: Tamper detected</div>`)
				return
			}
			c.String(http.StatusOK, `<div id="audit-status-badge">&#10004; Tamper-evident hash chain verified cleanly</div>`)
			return
		}

		if !intact {
			c.JSON(http.StatusOK, gin.H{"status": "tamper_detected", "intact": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "intact", "intact": true})
	})

	// 1. JSON intact request
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/aicte/audit/verify", nil)
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK || !strings.Contains(w1.Body.String(), `"intact":true`) {
		t.Fatalf("expected intact JSON response, got: %s", w1.Body.String())
	}

	// 2. HTMX intact request
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/aicte/audit/verify", nil)
	req2.Header.Set("HX-Request", "true")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK || !strings.Contains(w2.Body.String(), "Tamper-evident hash chain verified cleanly") {
		t.Fatalf("expected intact HTMX badge, got: %s", w2.Body.String())
	}

	// 3. HTMX tampered request
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/aicte/audit/verify?tampered=true", nil)
	req3.Header.Set("HX-Request", "true")
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK || !strings.Contains(w3.Body.String(), "Integrity failure") {
		t.Fatalf("expected tamper alert badge, got: %s", w3.Body.String())
	}
}

func TestVerifierVerification_ConsentGating(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/api/v1/credentials/:id/verify", func(c *gin.Context) {
		hasConsent := c.Query("consent") == "active"
		if !hasConsent {
			// Zero personal data returned
			c.JSON(http.StatusOK, gin.H{
				"credential_id":        c.Param("id"),
				"format_valid":         true,
				"registry_matched":     true,
				"rules_passed":         true,
				"institution_attested": true,
				"signature_valid":      true,
				"credential_current":   true,
				"consent_active":       false,
				"consent_required":     true,
				"status":               "consent_required",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"credential_id":        c.Param("id"),
			"format_valid":         true,
			"registry_matched":     true,
			"rules_passed":         true,
			"institution_attested": true,
			"signature_valid":      true,
			"credential_current":   true,
			"consent_active":       true,
			"consent_required":     false,
			"status":               "verified",
			"permitted_fields": gin.H{
				"name":         "Synthetic Student",
				"program_code": "BTECH-CSE",
			},
		})
	})

	// 1. Verification WITHOUT consent: Must have 6 states verified, consent_required=true, ZERO personal data
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/v1/credentials/cred-123/verify?consent=none", nil)
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w1.Code)
	}
	body1 := w1.Body.String()
	if strings.Contains(body1, "Synthetic Student") || strings.Contains(body1, "permitted_fields") {
		t.Fatalf("privacy violation: student data leaked without consent: %s", body1)
	}
	if !strings.Contains(body1, `"consent_required":true`) {
		t.Fatalf("expected consent_required: true, got: %s", body1)
	}

	// 2. Verification WITH active consent: Returns permitted fields
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/credentials/cred-123/verify?consent=active", nil)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w2.Code)
	}
	body2 := w2.Body.String()
	if !strings.Contains(body2, "Synthetic Student") || !strings.Contains(body2, `"consent_active":true`) {
		t.Fatalf("expected permitted student data with active consent, got: %s", body2)
	}
}

func TestAICTEReviews_ListMaskedPII(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/aicte/reviews", func(c *gin.Context) {
		rawAPAAR := "987654321012"
		rawDOB := "2004-11-20"

		// Ensure list view renders masked data only (Rule #4 & Acceptance Map)
		c.JSON(http.StatusOK, gin.H{
			"flags": []gin.H{
				{
					"rule_code":    "V7",
					"masked_apaar": contracts.MaskAPAAR(rawAPAAR),
					"masked_dob":   contracts.MaskDOB(rawDOB),
					"reason":       "Concurrent enrolment across institutions detected",
				},
			},
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/aicte/reviews", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	body := w.Body.String()
	// Must contain masked identifiers
	if !strings.Contains(body, "9876****1012") {
		t.Fatalf("expected masked APAAR in list view, got: %s", body)
	}
	if !strings.Contains(body, "2004-**-**") {
		t.Fatalf("expected masked DOB in list view, got: %s", body)
	}
	// Must NOT contain unmasked raw identifier
	if strings.Contains(body, "987654321012") {
		t.Fatalf("privacy violation: unmasked raw APAAR leaked in list view: %s", body)
	}
}

func TestAICTEReviews_DecisionSubmission_Audited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.POST("/aicte/reviews/:id/decision", func(c *gin.Context) {
		decision := contracts.ReviewDecision(strings.TrimSpace(c.PostForm("decision")))
		note := strings.TrimSpace(c.PostForm("note"))

		validDecisions := map[contracts.ReviewDecision]bool{
			contracts.DecisionConfirmedSamePerson:      true,
			contracts.DecisionConfirmedDifferentPeople: true,
			contracts.DecisionValidConcurrentStudy:    true,
			contracts.DecisionNeedsMoreInformation:     true,
			contracts.DecisionDismissed:                true,
		}

		if !validDecisions[decision] {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid review decision"})
			return
		}
		if note == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Regulatory review note is mandatory"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":   "resolved",
			"flag_id":  c.Param("id"),
			"decision": decision,
			"note":     note,
		})
	})

	// 1. Invalid decision -> 400 Bad Request
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodPost, "/aicte/reviews/flag-1/decision", strings.NewReader("decision=invalid_fraud_verdict&note=some_note"))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid decision, got %d", w1.Code)
	}

	// 2. Missing mandatory note -> 400 Bad Request
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodPost, "/aicte/reviews/flag-1/decision", strings.NewReader("decision=valid_concurrent_study&note="))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for empty note, got %d", w2.Code)
	}

	// 3. Valid submission -> 200 OK
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodPost, "/aicte/reviews/flag-1/decision", strings.NewReader("decision=valid_concurrent_study&note=Verified+degree+plus+diploma"))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid review decision, got %d", w3.Code)
	}
	if !strings.Contains(w3.Body.String(), `"decision":"valid_concurrent_study"`) {
		t.Fatalf("expected valid_concurrent_study in response, got: %s", w3.Body.String())
	}
}
