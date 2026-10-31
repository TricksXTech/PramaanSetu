package validate_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"apaar-verify/internal/contracts"
	"apaar-verify/internal/upload"
	"apaar-verify/internal/validate"
	"github.com/google/uuid"
)

type GoldenInstitutionStats struct {
	Total       int            `json:"total"`
	Outcomes    map[string]int `json:"outcomes"`
	FlagsByRule map[string]int `json:"flags_by_rule"`
}

type GoldenReport struct {
	FixtureFile    string                            `json:"fixture_file"`
	TotalRows      int                               `json:"total_rows"`
	Institutions   map[string]GoldenInstitutionStats `json:"institutions"`
	OverallSummary map[string]int                    `json:"overall_summary"`
}

func TestGoldenFixtureValidation_ExactCounts(t *testing.T) {
	// Dev B Task 1: Full fixture end-to-end assert of exact expected outcomes in a golden test
	fixturePath := filepath.Join("..", "..", "scripts", "fixtures", "test_admissions.csv")
	csvBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture file: %v", err)
	}

	reader := bytes.NewReader(csvBytes)
	parsedRows, err := upload.ParseCSVStream(reader)
	if err != nil {
		t.Fatalf("failed to parse fixture CSV stream: %v", err)
	}

	adapter := validate.NewMockRegistryAdapter()
	engine := validate.NewEngine(adapter)
	policy := validate.LoadPolicyConfig("")

	jobID := uuid.New()
	instID := "INST-001"

	instStats := GoldenInstitutionStats{
		Outcomes:    make(map[string]int),
		FlagsByRule: make(map[string]int),
	}
	overallSummary := map[string]int{
		"accepted":         0,
		"flagged_held":     0,
		"rejected_invalid": 0,
		"pending_registry": 0,
		"no_op_identical":  0,
		"conflict_stale":   0,
	}

	seenInFile := make(map[string]int)

	for _, pRow := range parsedRows {
		meta := &contracts.ValidationMetadata{
			JobID:         jobID,
			InstitutionID: instID,
			UploadRowID:   uuid.New(),
			RowNumber:     pRow.RowNumber,
			SeenInFile:    seenInFile,
			PolicyConfig:  &policy,
		}

		flags, outcome, err := engine.Validate(context.Background(), nil, pRow.NormalizedRow, meta)
		if err != nil {
			t.Fatalf("row %d validation unexpected error: %v", pRow.RowNumber, err)
		}

		outcomeStr := string(outcome)
		instStats.Total++
		instStats.Outcomes[outcomeStr]++
		overallSummary[outcomeStr]++

		for _, f := range flags {
			instStats.FlagsByRule[f.RuleCode]++
		}
	}

	actualReport := GoldenReport{
		FixtureFile: "scripts/fixtures/test_admissions.csv",
		TotalRows:   len(parsedRows),
		Institutions: map[string]GoldenInstitutionStats{
			instID: instStats,
		},
		OverallSummary: overallSummary,
	}

	// Load expected golden snapshot
	goldenPath := filepath.Join("..", "..", "scripts", "fixtures", "golden_expected.json")
	expectedBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden expected file: %v", err)
	}

	var expectedReport GoldenReport
	if err := json.Unmarshal(expectedBytes, &expectedReport); err != nil {
		t.Fatalf("failed to parse golden expected JSON: %v", err)
	}

	// Compare actual with golden snapshot
	if actualReport.TotalRows != expectedReport.TotalRows {
		t.Errorf("total rows mismatch: actual %d, expected %d", actualReport.TotalRows, expectedReport.TotalRows)
	}

	if !reflect.DeepEqual(actualReport.OverallSummary, expectedReport.OverallSummary) {
		t.Errorf("overall summary mismatch:\nActual:   %+v\nExpected: %+v", actualReport.OverallSummary, expectedReport.OverallSummary)
	}

	actualInst := actualReport.Institutions[instID]
	expectedInst := expectedReport.Institutions[instID]

	if !reflect.DeepEqual(actualInst.Outcomes, expectedInst.Outcomes) {
		t.Errorf("institution outcomes mismatch:\nActual:   %+v\nExpected: %+v", actualInst.Outcomes, expectedInst.Outcomes)
	}

	if !reflect.DeepEqual(actualInst.FlagsByRule, expectedInst.FlagsByRule) {
		t.Errorf("flags by rule mismatch:\nActual:   %+v\nExpected: %+v", actualInst.FlagsByRule, expectedInst.FlagsByRule)
	}
}
