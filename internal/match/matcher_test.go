package match_test

import (
	"testing"

	"apaar-verify/internal/contracts"
	"apaar-verify/internal/match"
)

func TestNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  Rahul  Kumar  ", "rahul kumar"},
		{"DR. A. P. J. ABDUL KALAM", "dr a p j abdul kalam"},
		{"Priya-Sharma", "priya sharma"},
		{"O'Connor", "oconnor"},
		{"Mohammad   Ali", "mohammad ali"},
	}

	for _, tc := range tests {
		got := match.Normalize(tc.input)
		if got != tc.expected {
			t.Errorf("Normalize(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestTrigramSimilarity(t *testing.T) {
	// Identical strings -> 1.0
	scoreExact := match.TrigramSimilarity("Aarav Sharma", "Aarav Sharma")
	if scoreExact != 1.0 {
		t.Errorf("expected 1.0 for identical strings, got %f", scoreExact)
	}

	// Minor typo: "Aarav Sharma" vs "Arav Sharma"
	scoreTypo := match.TrigramSimilarity("Aarav Sharma", "Arav Sharma")
	if scoreTypo < 0.65 {
		t.Errorf("expected score >= 0.65 for minor typo, got %f", scoreTypo)
	}

	// Different names: "Aarav Sharma" vs "Vikramaditya Rao"
	scoreDiff := match.TrigramSimilarity("Aarav Sharma", "Vikramaditya Rao")
	if scoreDiff > 0.3 {
		t.Errorf("expected low similarity (< 0.3) for distinct names, got %f", scoreDiff)
	}
}

func TestMatchConfig_Defaults(t *testing.T) {
	cfg := contracts.DefaultMatchConfig()
	if cfg.MinScore <= 0 || cfg.MinScore > 1.0 {
		t.Errorf("invalid default min_score: %f", cfg.MinScore)
	}
	if cfg.MaxCandidatesPerRow <= 0 {
		t.Errorf("invalid default max_candidates_per_row: %d", cfg.MaxCandidatesPerRow)
	}
	if cfg.QueryTimeoutMs <= 0 {
		t.Errorf("invalid default query_timeout_ms: %d", cfg.QueryTimeoutMs)
	}
	if len(cfg.BlockingStrategies) == 0 {
		t.Errorf("expected at least one blocking strategy")
	}
}

func TestMaskingFunctions(t *testing.T) {
	maskedAPAAR := contracts.MaskAPAAR("123456789012")
	if maskedAPAAR != "1234****9012" {
		t.Errorf("expected 1234****9012, got %s", maskedAPAAR)
	}

	maskedDOB := contracts.MaskDOB("2004-05-18")
	if maskedDOB != "2004-**-**" {
		t.Errorf("expected 2004-**-**, got %s", maskedDOB)
	}

	maskedName := contracts.MaskName("Amit Kumar Sharma")
	if maskedName != "A*** K*** S***" {
		t.Errorf("expected A*** K*** S***, got %s", maskedName)
	}
}

func TestFuzzyMatchingSanity_ThresholdsAndBounds(t *testing.T) {
	// Dev B Task 5:
	// 1. False-positive pair (same DOB, unrelated names) stays strictly below threshold
	unrelated1 := "Siddharth Malhotra"
	unrelated2 := "Aditya Vikram Joshi"
	scoreUnrelated := match.TrigramSimilarity(unrelated1, unrelated2)
	cfg := contracts.DefaultMatchConfig()

	if scoreUnrelated >= cfg.MinScore {
		t.Fatalf("false positive violation: unrelated pair (%s, %s) scored %f, which exceeds threshold %f",
			unrelated1, unrelated2, scoreUnrelated, cfg.MinScore)
	}

	// 2. Spelling-variant pair ("Aarav Sharma" vs "Arav Sharma") is flagged >= threshold
	variant1 := "Aarav Sharma"
	variant2 := "Arav Sharma"
	scoreVariant := match.TrigramSimilarity(variant1, variant2)
	if scoreVariant < cfg.MinScore {
		t.Fatalf("spelling variant pair (%s, %s) expected high similarity (>= %f), got %f",
			variant1, variant2, cfg.MinScore, scoreVariant)
	}

	// 3. Block cap enforcement (max candidates per row)
	maxCap := cfg.MaxCandidatesPerRow
	if maxCap != 5 {
		t.Fatalf("expected bounded candidate cap of 5 per block (Rule #4), got %d", maxCap)
	}

	// 4. Query timeout enforcement
	if cfg.QueryTimeoutMs > 500 {
		t.Fatalf("query timeout %dms exceeds maximum safe bound (500ms)", cfg.QueryTimeoutMs)
	}
}

