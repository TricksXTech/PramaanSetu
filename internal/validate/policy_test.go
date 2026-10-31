package validate_test

import (
	"testing"

	"apaar-verify/internal/contracts"
	"apaar-verify/internal/validate"
	"github.com/google/uuid"
)

func TestRuleV6_PolicyChecks_TableDriven(t *testing.T) {
	cfg := validate.DefaultPolicy

	tests := []struct {
		name         string
		row          contracts.CSVRow
		expectFlag   bool
		expectedRule string
	}{
		{
			name: "Valid regular student within age window",
			row: contracts.CSVRow{
				Category:        "General",
				AdmissionType:   "Regular",
				DOB:             "2004-05-15", // 22 years old
				Nationality:     "INDIAN",
				IsSupernumerary: false,
			},
			expectFlag: false,
		},
		{
			name: "Foreign National with Indian nationality -> Contradiction",
			row: contracts.CSVRow{
				Category:          "General",
				AdmissionType:     "Regular",
				DOB:               "2004-05-15",
				IsForeignNational: true,
				Nationality:       "INDIAN",
			},
			expectFlag:   true,
			expectedRule: "V6",
		},
		{
			name: "Both NRI and Foreign National set -> Contradiction",
			row: contracts.CSVRow{
				Category:          "General",
				AdmissionType:     "Regular",
				DOB:               "2004-05-15",
				IsNRI:             true,
				IsForeignNational: true,
				Nationality:       "FRENCH",
			},
			expectFlag:   true,
			expectedRule: "V6",
		},
		{
			name: "OCI holder with Indian nationality -> Contradiction",
			row: contracts.CSVRow{
				Category:      "General",
				AdmissionType: "Regular",
				DOB:           "2004-05-15",
				OCIStatus:     true,
				Nationality:   "INDIAN",
			},
			expectFlag:   true,
			expectedRule: "V6",
		},
		{
			name: "Underage student (below min age 16)",
			row: contracts.CSVRow{
				Category:      "General",
				AdmissionType: "Regular",
				DOB:           "2020-01-01", // ~6 years old
				Nationality:   "INDIAN",
			},
			expectFlag:   true,
			expectedRule: "V6",
		},
		{
			name: "Overage student (above max age 40)",
			row: contracts.CSVRow{
				Category:      "General",
				AdmissionType: "Regular",
				DOB:           "1970-01-01", // ~56 years old
				Nationality:   "INDIAN",
			},
			expectFlag:   true,
			expectedRule: "V6",
		},
		{
			name: "Invalid category",
			row: contracts.CSVRow{
				Category:      "SpecialCategoryUnknown",
				AdmissionType: "Regular",
				DOB:           "2004-05-15",
				Nationality:   "INDIAN",
			},
			expectFlag:   true,
			expectedRule: "V6",
		},
		{
			name: "Invalid admission type",
			row: contracts.CSVRow{
				Category:      "General",
				AdmissionType: "UnknownAdmissionType",
				DOB:           "2004-05-15",
				Nationality:   "INDIAN",
			},
			expectFlag:   true,
			expectedRule: "V6",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta := &contracts.ValidationMetadata{
				UploadRowID:  uuid.New(),
				RowNumber:    1,
				PolicyConfig: &cfg,
			}

			flags, outcome := validate.ValidateV6(&tc.row, meta)

			if tc.expectFlag {
				if outcome != contracts.RowOutcomeFlaggedHeld {
					t.Errorf("expected outcome flagged_held, got %s", outcome)
				}
				if len(flags) == 0 {
					t.Fatalf("expected flags for test %q, got none", tc.name)
				}
				if flags[0].RuleCode != tc.expectedRule {
					t.Errorf("expected rule code %s, got %s", tc.expectedRule, flags[0].RuleCode)
				}
				// Verify policy_version is recorded on every flag (Rule #6 & Step 0)
				if flags[0].PolicyVersion != cfg.PolicyVersion {
					t.Errorf("expected policy version %s, got %s", cfg.PolicyVersion, flags[0].PolicyVersion)
				}
			} else {
				if outcome != contracts.RowOutcomeAccepted || len(flags) > 0 {
					t.Errorf("expected clean acceptance, got outcome %s flags %v", outcome, flags)
				}
			}
		})
	}
}

