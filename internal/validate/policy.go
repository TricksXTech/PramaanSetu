package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"apaar-verify/internal/contracts"
)

// DefaultPolicy provides a robust fallback PolicyConfig
var DefaultPolicy = contracts.PolicyConfig{
	PolicyVersion:         "AICTE-2024-V1",
	AllowedCategories:     []string{"General", "OBC", "SC", "ST", "EWS"},
	AllowedAdmissionTypes: []string{"Regular", "Lateral Entry", "Supernumerary", "Management"},
	MinAgeYears:           16,
	MaxAgeYears:           40,
	SupernumeraryAllowances: map[string]int{
		"NRI":     5,
		"OCI":     5,
		"FN":      5,
		"TFW":     5,
		"DEFENCE": 2,
	},
	RequiredCategoryFields: map[string][]string{
		"Foreign_National": {"nationality"},
	},
}

// LoadPolicyConfig reads PolicyConfig from file or returns DefaultPolicy
func LoadPolicyConfig(filePath string) contracts.PolicyConfig {
	if filePath == "" {
		filePath = "config/policy.json"
	}
	b, err := os.ReadFile(filePath)
	if err != nil {
		return DefaultPolicy
	}
	var cfg contracts.PolicyConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return DefaultPolicy
	}
	return cfg
}

// ValidateV6 enforces policy consistency against PolicyConfig (Rule V6)
// Non-negotiable Rule #6: Implement against PolicyConfig only; plain-language reason naming field and rule.
func ValidateV6(row *contracts.CSVRow, meta *contracts.ValidationMetadata) ([]contracts.Flag, contracts.RowOutcome) {
	cfg := DefaultPolicy
	if meta != nil && meta.PolicyConfig != nil {
		cfg = *meta.PolicyConfig
	}

	targetID := ""
	if meta != nil {
		targetID = meta.UploadRowID.String()
	}

	var flags []contracts.Flag

	addFlag := func(reason string, evidence map[string]any) {
		flags = append(flags, contracts.Flag{
			RuleCode:      "V6",
			Severity:      "error",
			Reason:        reason,
			Evidence:      evidence,
			TargetType:    contracts.TargetUploadRow,
			TargetID:      targetID,
			PolicyVersion: cfg.PolicyVersion,
		})
	}

	// 1. Allowed Categories Check
	if !containsCaseInsensitive(cfg.AllowedCategories, row.Category) {
		addFlag(
			fmt.Sprintf("Category %q is invalid under policy %s", row.Category, cfg.PolicyVersion),
			map[string]any{"field": "category", "submitted": row.Category, "allowed": cfg.AllowedCategories},
		)
	}

	// 2. Allowed Admission Types Check
	if !containsCaseInsensitive(cfg.AllowedAdmissionTypes, row.AdmissionType) {
		addFlag(
			fmt.Sprintf("Admission type %q is invalid under policy %s", row.AdmissionType, cfg.PolicyVersion),
			map[string]any{"field": "admission_type", "submitted": row.AdmissionType, "allowed": cfg.AllowedAdmissionTypes},
		)
	}

	// 3. DOB Sanity & Age Window Check
	if row.DOB != "" {
		t, err := time.Parse("2006-01-02", row.DOB)
		if err != nil {
			t, err = time.Parse("02-01-2006", row.DOB)
		}
		if err == nil {
			now := time.Now().UTC()
			age := now.Year() - t.Year()
			if now.YearDay() < t.YearDay() {
				age--
			}
			if age < cfg.MinAgeYears || age > cfg.MaxAgeYears {
				addFlag(
					fmt.Sprintf("Student age (%d) is outside statutory limits [%d, %d] years under policy %s", age, cfg.MinAgeYears, cfg.MaxAgeYears, cfg.PolicyVersion),
					map[string]any{"field": "dob", "calculated_age": age, "min_age": cfg.MinAgeYears, "max_age": cfg.MaxAgeYears},
				)
			}
		}
	}

	// 4. Nationality & Foreign National (FN) Contradictions
	if row.IsForeignNational {
		if row.Nationality == "" || strings.EqualFold(row.Nationality, "INDIAN") {
			addFlag(
				fmt.Sprintf("Foreign National (FN) candidate cannot have Indian or missing nationality under policy %s", cfg.PolicyVersion),
				map[string]any{"field": "nationality", "is_foreign_national": true, "nationality": row.Nationality},
			)
		}
	}

	// 5. Mutual Exclusivity: NRI and FN both set
	if row.IsNRI && row.IsForeignNational {
		addFlag(
			fmt.Sprintf("Contradictory status: candidate cannot be tagged as both NRI and Foreign National under policy %s", cfg.PolicyVersion),
			map[string]any{"field": "is_nri", "is_foreign_national": true, "is_nri": true},
		)
	}

	// 6. OCI Status Consistency: OCI cannot have Indian nationality
	if row.OCIStatus {
		if row.Nationality == "" || strings.EqualFold(row.Nationality, "INDIAN") {
			addFlag(
				fmt.Sprintf("Overseas Citizen of India (OCI) candidate must have foreign nationality specified under policy %s", cfg.PolicyVersion),
				map[string]any{"field": "oci_status", "nationality": row.Nationality},
			)
		}
	}

	// 7. Supernumerary Flag without Qualifying Category
	if row.IsSupernumerary && !row.IsNRI && !row.OCIStatus && !row.IsForeignNational && !strings.EqualFold(row.AdmissionType, "Supernumerary") {
		addFlag(
			fmt.Sprintf("Supernumerary flag requires a qualifying category (NRI, OCI, FN) or Supernumerary admission type under policy %s", cfg.PolicyVersion),
			map[string]any{"field": "is_supernumerary", "admission_type": row.AdmissionType},
		)
	}

	if len(flags) > 0 {
		return flags, contracts.RowOutcomeFlaggedHeld
	}

	return nil, contracts.RowOutcomeAccepted
}

func containsCaseInsensitive(list []string, item string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(item)) {
			return true
		}
	}
	return false
}

