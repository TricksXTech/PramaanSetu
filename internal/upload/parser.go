package upload

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"apaar-verify/internal/contracts"
)

var (
	ErrFileTooLarge     = errors.New("file exceeds maximum allowed size (10MB)")
	ErrTooManyRows      = errors.New("file exceeds maximum allowed row limit (20000 rows)")
	ErrFieldTooLarge    = errors.New("field value exceeds maximum allowed size (500 characters)")
	ErrEmptyFile        = errors.New("uploaded file is empty")
	ErrMissingHeaders   = errors.New("missing required CSV header columns")
	ErrInvalidHeaderRow = errors.New("invalid CSV header structure")
)

const (
	MaxFileSizeBytes = 10 * 1024 * 1024 // 10 MB
	MaxRowCount      = 20000
	MaxFieldLength   = 500
)

// RequiredHeaders defines the canonical header names
var RequiredHeaders = []string{
	"apaar_id",
	"name",
	"dob",
	"program_id",
	"academic_year",
	"admission_reference",
}

// ParsedRowData contains raw row, normalized contracts.CSVRow, and parse errors
type ParsedRowData struct {
	RowNumber        int
	RawPayload       map[string]string
	NormalizedRow    *contracts.CSVRow
	NormalizedFields map[string]any
	ParseErrors      []string
	Outcome          contracts.RowOutcome
}

// SanitizeCSVValue prevents spreadsheet formula injection (Rule #8)
// Any cell starting with =, +, -, @, \t, or \r is prepended with a quote '
func SanitizeCSVValue(val string) string {
	trimmed := strings.TrimSpace(val)
	if len(trimmed) == 0 {
		return ""
	}
	firstChar := trimmed[0]
	if firstChar == '=' || firstChar == '+' || firstChar == '-' || firstChar == '@' || firstChar == '\t' || firstChar == '\r' {
		return "'" + trimmed
	}
	return trimmed
}

// ParseCSVStream parses an incoming CSV reader with strict streaming bounds
func ParseCSVStream(r io.Reader) ([]ParsedRowData, error) {
	csvReader := csv.NewReader(r)
	csvReader.TrimLeadingSpace = true
	csvReader.ReuseRecord = false

	headers, err := csvReader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, ErrEmptyFile
		}
		return nil, err
	}

	headerMap := make(map[string]int)
	for i, h := range headers {
		clean := strings.ToLower(strings.TrimSpace(h))
		headerMap[clean] = i
	}

	// Check required headers
	for _, req := range RequiredHeaders {
		if _, ok := headerMap[req]; !ok {
			return nil, ErrMissingHeaders
		}
	}

	var parsedRows []ParsedRowData
	rowNum := 1 // Header is row 1, data starts at row 2

	for {
		record, err := csvReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			rowNum++
			// Non-negotiable Rule #3: Every row preserved, including parse failures!
			parsedRows = append(parsedRows, ParsedRowData{
				RowNumber:   rowNum,
				RawPayload:  map[string]string{"raw_line": strings.Join(record, ",")},
				ParseErrors: []string{err.Error()},
				Outcome:     contracts.RowOutcomeRejectedParse,
			})
			continue
		}

		rowNum++
		if rowNum > MaxRowCount+1 {
			return nil, ErrTooManyRows
		}

		rawMap := make(map[string]string)
		var parseErrs []string

		for h, idx := range headerMap {
			if idx < len(record) {
				val := record[idx]
				if len(val) > MaxFieldLength {
					parseErrs = append(parseErrs, "Field "+h+" exceeds maximum size limit (500 chars)")
					val = val[:MaxFieldLength]
				}
				rawMap[h] = SanitizeCSVValue(val)
			} else {
				rawMap[h] = ""
			}
		}

		apaarID := strings.TrimSpace(rawMap["apaar_id"])
		name := strings.TrimSpace(rawMap["name"])
		dobStr := strings.TrimSpace(rawMap["dob"])
		progID := strings.TrimSpace(rawMap["program_id"])
		acadYear := strings.TrimSpace(rawMap["academic_year"])
		admRef := strings.TrimSpace(rawMap["admission_reference"])

		// Check basic required fields for parse rejection
		if name == "" {
			parseErrs = append(parseErrs, "Missing student name")
		}
		if dobStr == "" {
			parseErrs = append(parseErrs, "Missing date of birth")
		} else {
			if _, err := time.Parse("2006-01-02", dobStr); err != nil {
				if _, err2 := time.Parse("02-01-2006", dobStr); err2 != nil {
					parseErrs = append(parseErrs, "Invalid date format, expected YYYY-MM-DD")
				}
			}
		}
		if progID == "" {
			parseErrs = append(parseErrs, "Missing program code")
		}
		if acadYear == "" {
			parseErrs = append(parseErrs, "Missing academic year")
		}
		if admRef == "" {
			parseErrs = append(parseErrs, "Missing admission reference")
		}

		csvRow := &contracts.CSVRow{
			APAARID:            apaarID,
			StudentName:        name,
			DOB:                dobStr,
			Gender:             strings.TrimSpace(rawMap["gender"]),
			GuardianName:       strings.TrimSpace(rawMap["guardian_name"]),
			ProgramCode:        progID,
			AcademicYear:       acadYear,
			AdmissionReference: admRef,
			Category:           getOrDefault(rawMap["category"], "General"),
			AdmissionType:      getOrDefault(rawMap["admission_type"], "Regular"),
			Nationality:        strings.ToUpper(getOrDefault(rawMap["nationality"], "INDIAN")),
			IsSupernumerary:    parseBool(rawMap["is_supernumerary"]),
			IsNRI:              parseBool(rawMap["is_nri"]),
			OCIStatus:          parseBool(rawMap["oci_status"]),
			IsForeignNational:  parseBool(rawMap["is_foreign_national"]),
		}

		normalizedFields := map[string]any{
			"apaar_id":            csvRow.APAARID,
			"student_name":        csvRow.StudentName,
			"dob":                 csvRow.DOB,
			"gender":              csvRow.Gender,
			"guardian_name":       csvRow.GuardianName,
			"program_code":        csvRow.ProgramCode,
			"academic_year":       csvRow.AcademicYear,
			"admission_reference": csvRow.AdmissionReference,
			"category":            csvRow.Category,
			"admission_type":      csvRow.AdmissionType,
			"nationality":         csvRow.Nationality,
			"is_supernumerary":    csvRow.IsSupernumerary,
			"is_nri":              csvRow.IsNRI,
			"oci_status":          csvRow.OCIStatus,
			"is_foreign_national": csvRow.IsForeignNational,
		}

		outcome := contracts.RowOutcome("staged")
		if len(parseErrs) > 0 {
			// Row rejected at parse stage (Non-negotiable Rule #3)
			outcome = contracts.RowOutcomeRejectedParse
		}

		parsedRows = append(parsedRows, ParsedRowData{
			RowNumber:        rowNum,
			RawPayload:       rawMap,
			NormalizedRow:    csvRow,
			NormalizedFields: normalizedFields,
			ParseErrors:      parseErrs,
			Outcome:          outcome,
		})
	}

	return parsedRows, nil
}

func parseBool(s string) bool {
	v := strings.ToLower(strings.TrimSpace(s))
	return v == "true" || v == "1" || v == "yes" || v == "y"
}

func getOrDefault(val, def string) string {
	t := strings.TrimSpace(val)
	if t == "" {
		return def
	}
	return t
}

func ToJSONBytes(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
