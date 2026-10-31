package validate_test

import (
	"context"
	"testing"

	"apaar-verify/internal/contracts"
	"apaar-verify/internal/validate"
	"github.com/google/uuid"
)

func TestRuleV1_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		apaarID         string
		expectedOutcome contracts.RowOutcome
		shouldFlag      bool
	}{
		{"Valid 12 digits", "123456789012", contracts.RowOutcomeAccepted, false},
		{"Valid leading zeros", "000123456789", contracts.RowOutcomeAccepted, false},
		{"Too short 11 digits", "12345678901", contracts.RowOutcomeRejectedInvalid, true},
		{"Too long 13 digits", "1234567890123", contracts.RowOutcomeRejectedInvalid, true},
		{"Contains alphabetic letters", "12345678901A", contracts.RowOutcomeRejectedInvalid, true},
		{"Contains Unicode digits (e.g. Devanagari)", "१२३४५६७८९०१२", contracts.RowOutcomeRejectedInvalid, true},
		{"Contains internal whitespace", "1234 5678 901", contracts.RowOutcomeRejectedInvalid, true},
		{"Empty string", "", contracts.RowOutcomeRejectedInvalid, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			row := &contracts.CSVRow{
				APAARID: tc.apaarID,
			}
			meta := &contracts.ValidationMetadata{
				UploadRowID: uuid.New(),
				RowNumber:   1,
			}

			flags, outcome := validate.ValidateV1(row, meta)
			if outcome != tc.expectedOutcome {
				t.Errorf("expected outcome %q, got %q", tc.expectedOutcome, outcome)
			}
			if tc.shouldFlag && len(flags) == 0 {
				t.Errorf("expected V1 flag for test %q, got none", tc.name)
			}
			if !tc.shouldFlag && len(flags) > 0 {
				t.Errorf("unexpected flag for valid case: %v", flags)
			}
		})
	}
}

func TestRuleV2_RegistryAdapter(t *testing.T) {
	adapter := validate.NewMockRegistryAdapter()
	engine := validate.NewEngine(adapter)

	tests := []struct {
		name            string
		apaarID         string
		expectedOutcome contracts.RowOutcome
		expectedFlag    string
	}{
		{"Matched synthetic record", "123456789012", contracts.RowOutcomeAccepted, ""},
		{"Unavailable produces pending_registry (Rule 6)", "555512345678", contracts.RowOutcomePendingRegistry, "V2"},
		{"Unavailable suffix 503 produces pending_registry", "123456789503", contracts.RowOutcomePendingRegistry, "V2"},
		{"Not found produces flagged_held", "000012345678", contracts.RowOutcomeFlaggedHeld, "V2"},
		{"Not found suffix 404 produces flagged_held", "123456789404", contracts.RowOutcomeFlaggedHeld, "V2"},
		{"Identity mismatch suffix 999 produces flagged_held", "123456789999", contracts.RowOutcomeFlaggedHeld, "V2"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			row := &contracts.CSVRow{
				APAARID: tc.apaarID,
			}
			meta := &contracts.ValidationMetadata{
				UploadRowID: uuid.New(),
				RowNumber:   2,
			}

			flags, outcome, err := engine.ValidateV2(context.Background(), row, meta)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if outcome != tc.expectedOutcome {
				t.Errorf("expected outcome %q, got %q", tc.expectedOutcome, outcome)
			}
			if tc.expectedFlag != "" {
				if len(flags) == 0 || flags[0].RuleCode != tc.expectedFlag {
					t.Errorf("expected flag %s, got %v", tc.expectedFlag, flags)
				}
				// Verify honest mock provenance is recorded
				if prov, ok := flags[0].Evidence["provenance"].(string); !ok || prov != "mock" {
					t.Errorf("expected mock provenance in flag evidence, got %v", flags[0].Evidence)
				}
			}
		})
	}
}

func TestRuleV3_RepeatedWithinFile(t *testing.T) {
	seenMap := make(map[string]int)

	row1 := &contracts.CSVRow{
		APAARID:            "111122223333",
		ProgramCode:        "BTECH-CSE",
		AdmissionReference: "REF-001",
	}
	meta1 := &contracts.ValidationMetadata{
		UploadRowID: uuid.New(),
		RowNumber:   1,
		SeenInFile:  seenMap,
	}

	// First occurrence: must pass
	flags1, outcome1 := validate.ValidateV3(row1, meta1)
	if outcome1 != contracts.RowOutcomeAccepted || len(flags1) != 0 {
		t.Fatalf("first occurrence should be accepted, got outcome %v flags %v", outcome1, flags1)
	}

	// Identical repeated submission in same file (Row 5)
	meta2 := &contracts.ValidationMetadata{
		UploadRowID: uuid.New(),
		RowNumber:   5,
		SeenInFile:  seenMap,
	}
	flags2, outcome2 := validate.ValidateV3(row1, meta2)
	if outcome2 != contracts.RowOutcomeFlaggedHeld {
		t.Fatalf("repeated submission should be flagged_held, got %v", outcome2)
	}
	if len(flags2) == 0 || flags2[0].RuleCode != "V3" {
		t.Fatalf("expected V3 flag, got %v", flags2)
	}
}

