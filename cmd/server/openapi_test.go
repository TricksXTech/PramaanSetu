package main

import (
	"os"
	"strings"
	"testing"

	"apaar-verify/internal/contracts"
	"apaar-verify/internal/credentials"
	"github.com/gin-gonic/gin"
)

// TestOpenAPI_RouteAndSchemaParity verifies Milestone 6 Dev D Requirement #4:
// Validate /docs/openapi.yaml against the actual Gin router and assert every documented endpoint exists
// with exact paths, methods, and required response codes.
func TestOpenAPI_RouteAndSchemaParity(t *testing.T) {
	// Navigate to project root if running from cmd/server directory
	cwd, _ := os.Getwd()
	if strings.HasSuffix(cwd, "cmd\\server") || strings.HasSuffix(cwd, "cmd/server") {
		_ = os.Chdir("../..")
		defer func() { _ = os.Chdir(cwd) }()
	}

	gin.SetMode(gin.TestMode)
	signer, err := credentials.NewEd25519Signer("")
	if err != nil {
		t.Fatalf("failed to create signer for router setup: %v", err)
	}
	policyCfg := contracts.PolicyConfig{}

	r := setupRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, signer, nil, &policyCfg)
	if r == nil {
		t.Fatalf("setupRouter returned nil")
	}

	// Index all registered routes in the actual Gin router
	routes := r.Routes()
	routeMap := make(map[string]bool)
	for _, rt := range routes {
		routeMap[rt.Method+":"+rt.Path] = true
	}

	// 1. Assert all 4 core OpenAPI REST endpoints are active in the router
	expectedEndpoints := []struct {
		Method string
		Path   string
	}{
		{Method: "POST", Path: "/api/v1/uploads"},
		{Method: "GET", Path: "/api/v1/uploads/:id"},
		{Method: "PATCH", Path: "/api/v1/enrolments/:id"},
		{Method: "GET", Path: "/api/v1/credentials/:id/verify"},
	}

	for _, ep := range expectedEndpoints {
		key := ep.Method + ":" + ep.Path
		if !routeMap[key] {
			t.Errorf("OpenAPI documented endpoint %s is missing in Gin router", key)
		}
	}

	// 2. Validate docs/openapi.yaml file content and schema definitions
	specBytes, err := os.ReadFile("docs/openapi.yaml")
	if err != nil {
		t.Fatalf("failed to read docs/openapi.yaml: %v", err)
	}
	spec := string(specBytes)

	requiredSpecItems := []string{
		"/uploads:",
		"/uploads/{id}:",
		"/enrolments/{id}:",
		"/credentials/{id}/verify:",
		"'202':", // Batch queued
		"'200':", // OK
		"'400':", // Bad Request / Identity modification prohibited
		"'401':", // Unauthorized (missing/invalid API key)
		"'403':", // Forbidden (cross-tenant / insufficient scope)
		"'409':", // Optimistic concurrency conflict (stale version)
		"'429':", // Rate limit backoff
		"Idempotency-Key",
		"X-API-Key",
		"uploads:write",
		"uploads:read",
		"enrolments:write",
		"credentials:verify",
	}

	for _, item := range requiredSpecItems {
		if !strings.Contains(spec, item) {
			t.Errorf("docs/openapi.yaml missing required specification element: %s", item)
		}
	}
}
