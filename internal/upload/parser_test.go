package upload_test

import (
	"os"
	"strings"
	"testing"

	"apaar-verify/internal/contracts"
	"apaar-verify/internal/upload"
	"apaar-verify/internal/validate"
)

func TestSanitizeCSVValue_FormulaInjection(t *testing.T) {
	// Formula injection vectors (Rule #8)
	malicious := []string{
		"=SUM(A1:A10)",
		"+cmd|' /C calc'!A0",
		"-1337",
		"@SUM(1+1)",
		"\t=1+1",
	}

	for _, input := range malicious {
		sanitized := upload.SanitizeCSVValue(input)
		if !strings.HasPrefix(sanitized, "'") {
			t.Errorf("expected formula input %q to be escaped with leading quote, got %q", input, sanitized)
		}
	}

	// Safe inputs
	safe := []string{"Rahul Verma", "DTU-2024-001", "123456789012"}
	for _, input := range safe {
		if upload.SanitizeCSVValue(input) != input {
			t.Errorf("safe value %q was modified unexpectedly", input)
		}
	}
}

func TestParseCSVStream_FixtureValidation(t *testing.T) {
	fixtureData, err := os.ReadFile("../../scripts/fixtures/test_admissions.csv")
	if err != nil {
		t.Fatalf("failed to read fixture CSV: %v", err)
	}

	rows, err := upload.ParseCSVStream(strings.NewReader(string(fixtureData)))
	if err != nil {
		t.Fatalf("failed to parse CSV stream: %v", err)
	}

	if len(rows) != 7 {
		t.Fatalf("expected 7 rows parsed from fixture, got %d", len(rows))
	}

	// Row 1 (row_number = 2): Malformed APAAR ID "9999123"
	// Rule V1 checks this during validation
	row1 := rows[0]
	flags, outcome := validate.ValidateV1(row1.NormalizedRow, nil)
	if outcome != contracts.RowOutcomeRejectedInvalid {
		t.Errorf("expected V1 to reject row 1 with rejected_invalid, got %s", outcome)
	}
	if len(flags) == 0 || flags[0].RuleCode != "V1" {
		t.Errorf("expected V1 flag for row 1, got %v", flags)
	}

	// Row 2 & 3: Repeated submission
	row2 := rows[1]
	row3 := rows[2]
	if row2.NormalizedRow.APAARID != row3.NormalizedRow.APAARID {
		t.Errorf("expected row 2 and 3 to have identical APAAR ID for duplicate test")
	}

	// Row 7 (row_number = 8): Student for cross-institution test
	row7 := rows[6]
	if row7.NormalizedRow.APAARID != "987654321098" {
		t.Errorf("expected row 7 to have APAAR 987654321098, got %s", row7.NormalizedRow.APAARID)
	}
}
