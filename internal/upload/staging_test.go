package upload_test

import (
	"strings"
	"testing"

	"apaar-verify/internal/contracts"
	"apaar-verify/internal/upload"
)

func TestParseCSVStream_ParseFailuresPreserved(t *testing.T) {
	// CSV containing:
	// 1. Valid row
	// 2. Syntax/parse failure (missing required columns/fields)
	csvContent := `apaar_id,name,dob,program_id,academic_year,admission_reference
123456789012,Ananya Patel,2004-06-25,BTECH-CSE,2024-25,REF-001
123456789013,,2004-06-25,BTECH-CSE,2024-25,REF-002
`
	rows, err := upload.ParseCSVStream(strings.NewReader(csvContent))
	if err != nil {
		t.Fatalf("unexpected error parsing stream: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Row 1: Valid format -> staged
	if rows[0].Outcome != "staged" {
		t.Errorf("expected row 1 outcome 'staged', got %s", rows[0].Outcome)
	}

	// Row 2: Missing student name -> rejected_parse (Rule #3: preserved!)
	if rows[1].Outcome != contracts.RowOutcomeRejectedParse {
		t.Errorf("expected row 2 outcome 'rejected_parse', got %s", rows[1].Outcome)
	}
	if len(rows[1].ParseErrors) == 0 {
		t.Errorf("expected parse errors recorded for row 2, got none")
	}
}

func TestParseCSVStream_FieldSizeBounds(t *testing.T) {
	// Value exceeding 500 chars
	giantName := strings.Repeat("A", 600)
	csvContent := `apaar_id,name,dob,program_id,academic_year,admission_reference
123456789012,` + giantName + `,2004-06-25,BTECH-CSE,2024-25,REF-001
`
	rows, err := upload.ParseCSVStream(strings.NewReader(csvContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	// Must truncate or flag field size violation
	if len(rows[0].ParseErrors) == 0 {
		t.Errorf("expected field size limit violation recorded in parse errors")
	}
}

