package validate

import (
	"context"
	"strings"

	"apaar-verify/internal/contracts"
)

// MockRegistryAdapter implements contracts.RegistryAdapter with explicit mock provenance
type MockRegistryAdapter struct{}

func NewMockRegistryAdapter() *MockRegistryAdapter {
	return &MockRegistryAdapter{}
}

// Lookup simulates querying the national APAAR registry stub
// Honest Demo Claims: Always labelled as "mock" provenance
func (m *MockRegistryAdapter) Lookup(ctx context.Context, apaarID string) (contracts.RegistryResult, error) {
	trimmed := strings.TrimSpace(apaarID)

	// Controlled test stub behavior:
	// 1. Simulating temporary registry downtime / network timeout -> "unavailable"
	if strings.HasSuffix(trimmed, "503") || strings.HasPrefix(trimmed, "5555") {
		return contracts.RegistryResult{
			Status:     contracts.RegistryStatusUnavailable,
			Provenance: "mock",
			Details:    map[string]any{"adapter": "mock_apaar_v1", "reason": "registry service temporarily unreachable"},
		}, nil
	}

	// 2. Simulating unissued / nonexistent APAAR ID -> "not_found"
	if strings.HasSuffix(trimmed, "404") || strings.HasPrefix(trimmed, "0000") {
		return contracts.RegistryResult{
			Status:     contracts.RegistryStatusNotFound,
			Provenance: "mock",
			Details:    map[string]any{"adapter": "mock_apaar_v1", "reason": "identifier not found in registry"},
		}, nil
	}

	// 3. Simulating name/DOB mismatch against registry -> "identity_mismatch"
	if strings.HasSuffix(trimmed, "999") {
		return contracts.RegistryResult{
			Status:     contracts.RegistryStatusIdentityMismatch,
			Provenance: "mock",
			Details:    map[string]any{"adapter": "mock_apaar_v1", "reason": "submitted details do not match registry record"},
		}, nil
	}

	// 4. Default valid synthetic registry match
	return contracts.RegistryResult{
		Status:     contracts.RegistryStatusMatched,
		Provenance: "mock",
		Details:    map[string]any{"adapter": "mock_apaar_v1", "verified_syntax": true},
	}, nil
}

