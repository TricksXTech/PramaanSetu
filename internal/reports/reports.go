package reports

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"apaar-verify/internal/contracts"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service provides aggregated regulatory reports and formula-safe CSV exports
type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// IntakeRow aggregates programme capacity vs enrolled counts
type IntakeRow struct {
	InstitutionID   string `json:"institution_id"`
	InstitutionName string `json:"institution_name"`
	ProgramID       string `json:"program_id"`
	AcademicYear    string `json:"academic_year"`
	RegularLimit    int    `json:"regular_limit"`
	AcceptedRegular int    `json:"accepted_regular"`
	Supernumerary   int    `json:"supernumerary"`
	NRI             int    `json:"nri"`
	OCI             int    `json:"oci"`
	FN              int    `json:"fn"`
	HeldCount       int    `json:"held_count"`
	Variance        int    `json:"variance"` // AcceptedRegular - RegularLimit
}

type IntakeReportResult struct {
	Spec  contracts.ReportSpec `json:"spec"`
	Rows  []IntakeRow          `json:"rows"`
	Total IntakeRow          `json:"total"`
}

// CategoryRow aggregates enrolment counts across social and admission categories
type CategoryRow struct {
	InstitutionID   string `json:"institution_id"`
	InstitutionName string `json:"institution_name"`
	ProgramID       string `json:"program_id"`
	AcademicYear    string `json:"academic_year"`
	Category        string `json:"category"`
	EnrolledCount   int    `json:"enrolled_count"`
}

type CategoryReportResult struct {
	Spec contracts.ReportSpec `json:"spec"`
	Rows []CategoryRow        `json:"rows"`
}

// FlaggedRow aggregates flagged review leads by rule and resolution state
type FlaggedRow struct {
	InstitutionID   string `json:"institution_id"`
	InstitutionName string `json:"institution_name"`
	RuleCode        string `json:"rule_code"`
	Severity        string `json:"severity"`
	ReviewStatus    string `json:"review_status"`
	Count           int    `json:"count"`
}

type FlaggedReportResult struct {
	Spec contracts.ReportSpec `json:"spec"`
	Rows []FlaggedRow         `json:"rows"`
}

// GenerateIntakeVsEnrolmentReport reads aggregates from rollup_enrolment (never raw student records)
func (s *Service) GenerateIntakeVsEnrolmentReport(
	ctx context.Context,
	institutionID *string,
	academicYear string,
	generatedBy string,
) (*IntakeReportResult, error) {
	if academicYear == "" {
		academicYear = "2024-25"
	}

	query := `
		SELECT ai.institution_id, COALESCE(i.name, ai.institution_id), ai.program_id, ai.academic_year,
		       ai.regular_limit, COALESCE(re.total_enrolled, 0), COALESCE(re.supernumerary, 0),
		       COALESCE(re.nri, 0), COALESCE(re.oci, 0), COALESCE(re.foreign_national, 0),
		       COALESCE(re.held_count, 0)
		FROM approved_intake ai
		LEFT JOIN institutions i ON ai.institution_id = i.id
		LEFT JOIN rollup_enrolment re ON ai.institution_id = re.institution_id
		                             AND ai.program_id = re.program_id
		                             AND ai.academic_year = re.academic_year
		WHERE ($1::varchar IS NULL OR ai.institution_id = $1)
		  AND ai.academic_year = $2
		ORDER BY ai.institution_id, ai.program_id
	`
	rows, err := s.db.Query(ctx, query, institutionID, academicYear)
	if err != nil {
		return nil, fmt.Errorf("failed to query intake report: %w", err)
	}
	defer rows.Close()

	var reportRows []IntakeRow
	var total IntakeRow
	total.InstitutionName = "TOTAL (AGGREGATE)"

	for rows.Next() {
		var r IntakeRow
		if err := rows.Scan(
			&r.InstitutionID, &r.InstitutionName, &r.ProgramID, &r.AcademicYear,
			&r.RegularLimit, &r.AcceptedRegular, &r.Supernumerary,
			&r.NRI, &r.OCI, &r.FN, &r.HeldCount,
		); err == nil {
			r.Variance = r.AcceptedRegular - r.RegularLimit
			reportRows = append(reportRows, r)

			total.RegularLimit += r.RegularLimit
			total.AcceptedRegular += r.AcceptedRegular
			total.Supernumerary += r.Supernumerary
			total.NRI += r.NRI
			total.OCI += r.OCI
			total.FN += r.FN
			total.HeldCount += r.HeldCount
			total.Variance += r.Variance
		}
	}

	scope := "national"
	if institutionID != nil {
		scope = *institutionID
	}

	return &IntakeReportResult{
		Spec: contracts.ReportSpec{
			ReportType:    contracts.ReportIntakeVsEnrolment,
			Scope:         scope,
			AcademicYear:  academicYear,
			GeneratedBy:   generatedBy,
			GeneratedAt:   time.Now().UTC(),
			PolicyVersion: "AICTE-2024-V1",
		},
		Rows:  reportRows,
		Total: total,
	}, nil
}

// GenerateCategoryBreakdownReport calculates category totals
func (s *Service) GenerateCategoryBreakdownReport(
	ctx context.Context,
	institutionID *string,
	academicYear string,
	generatedBy string,
) (*CategoryReportResult, error) {
	if academicYear == "" {
		academicYear = "2024-25"
	}

	query := `
		SELECT e.institution_id, COALESCE(i.name, e.institution_id), e.program_id, e.academic_year,
		       e.category, COUNT(*) AS cnt
		FROM enrolments e
		LEFT JOIN institutions i ON e.institution_id = i.id
		WHERE ($1::varchar IS NULL OR e.institution_id = $1)
		  AND e.academic_year = $2
		  AND e.status IN ('submitted', 'attested')
		GROUP BY e.institution_id, i.name, e.program_id, e.academic_year, e.category
		ORDER BY e.institution_id, e.program_id, e.category
	`
	rows, err := s.db.Query(ctx, query, institutionID, academicYear)
	if err != nil {
		return nil, fmt.Errorf("failed to query category report: %w", err)
	}
	defer rows.Close()

	var reportRows []CategoryRow
	for rows.Next() {
		var r CategoryRow
		if err := rows.Scan(&r.InstitutionID, &r.InstitutionName, &r.ProgramID, &r.AcademicYear, &r.Category, &r.EnrolledCount); err == nil {
			reportRows = append(reportRows, r)
		}
	}

	scope := "national"
	if institutionID != nil {
		scope = *institutionID
	}

	return &CategoryReportResult{
		Spec: contracts.ReportSpec{
			ReportType:    contracts.ReportCategoryBreakdown,
			Scope:         scope,
			AcademicYear:  academicYear,
			GeneratedBy:   generatedBy,
			GeneratedAt:   time.Now().UTC(),
			PolicyVersion: "AICTE-2024-V1",
		},
		Rows: reportRows,
	}, nil
}

// GenerateFlaggedSummaryReport groups open and resolved review leads by rule code
func (s *Service) GenerateFlaggedSummaryReport(
	ctx context.Context,
	institutionID *string,
	generatedBy string,
) (*FlaggedReportResult, error) {
	query := `
		SELECT f.institution_id, COALESCE(i.name, f.institution_id), f.rule_code, f.severity, f.review_status, COUNT(*)
		FROM validation_flags f
		LEFT JOIN institutions i ON f.institution_id = i.id
		WHERE ($1::varchar IS NULL OR f.institution_id = $1)
		GROUP BY f.institution_id, i.name, f.rule_code, f.severity, f.review_status
		ORDER BY f.institution_id, f.rule_code, f.review_status
	`
	rows, err := s.db.Query(ctx, query, institutionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query flagged summary: %w", err)
	}
	defer rows.Close()

	var reportRows []FlaggedRow
	for rows.Next() {
		var r FlaggedRow
		if err := rows.Scan(&r.InstitutionID, &r.InstitutionName, &r.RuleCode, &r.Severity, &r.ReviewStatus, &r.Count); err == nil {
			reportRows = append(reportRows, r)
		}
	}

	scope := "national"
	if institutionID != nil {
		scope = *institutionID
	}

	return &FlaggedReportResult{
		Spec: contracts.ReportSpec{
			ReportType:    contracts.ReportFlaggedSummary,
			Scope:         scope,
			AcademicYear:  "All",
			GeneratedBy:   generatedBy,
			GeneratedAt:   time.Now().UTC(),
			PolicyVersion: "AICTE-2024-V1",
		},
		Rows: reportRows,
	}, nil
}

// SanitizeCSVCell neutralizes spreadsheet formula injection (Rule #8 & Acceptance Map)
// Prefixes any cell starting with '=', '+', '-', '@', tab, or carriage return with a single quote
func SanitizeCSVCell(cell string) string {
	if len(cell) == 0 {
		return cell
	}
	first := cell[0]
	if first == '=' || first == '+' || first == '-' || first == '@' || first == '\t' || first == '\r' {
		return "'" + cell
	}
	return cell
}

// ExportCSV streams tabular records with UTF-8 BOM and formula injection protection
func ExportCSV(headers []string, records [][]string) ([]byte, error) {
	var buf bytes.Buffer

	// 1. Write UTF-8 BOM for Microsoft Excel compatibility
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(&buf)

	// 2. Sanitize and write headers
	sanitizedHeaders := make([]string, len(headers))
	for i, h := range headers {
		sanitizedHeaders[i] = SanitizeCSVCell(h)
	}
	if err := writer.Write(sanitizedHeaders); err != nil {
		return nil, err
	}

	// 3. Sanitize and write records
	for _, rec := range records {
		sanitizedRec := make([]string, len(rec))
		for i, cell := range rec {
			sanitizedRec[i] = SanitizeCSVCell(cell)
		}
		if err := writer.Write(sanitizedRec); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}

// ConvertIntakeReportToCSV converts IntakeReportResult to sanitized CSV bytes
func ConvertIntakeReportToCSV(rep *IntakeReportResult) ([]byte, error) {
	headers := []string{
		"Institution ID", "Institution Name", "Program", "Academic Year",
		"Approved Intake", "Accepted Regular", "Supernumerary", "NRI", "OCI", "FN",
		"Held (Cap Overflow)", "Variance",
	}

	var rows [][]string
	for _, r := range rep.Rows {
		rows = append(rows, []string{
			r.InstitutionID,
			r.InstitutionName,
			r.ProgramID,
			r.AcademicYear,
			strconv.Itoa(r.RegularLimit),
			strconv.Itoa(r.AcceptedRegular),
			strconv.Itoa(r.Supernumerary),
			strconv.Itoa(r.NRI),
			strconv.Itoa(r.OCI),
			strconv.Itoa(r.FN),
			strconv.Itoa(r.HeldCount),
			strconv.Itoa(r.Variance),
		})
	}

	// Append Total row
	rows = append(rows, []string{
		"TOTAL",
		rep.Total.InstitutionName,
		"-",
		rep.Spec.AcademicYear,
		strconv.Itoa(rep.Total.RegularLimit),
		strconv.Itoa(rep.Total.AcceptedRegular),
		strconv.Itoa(rep.Total.Supernumerary),
		strconv.Itoa(rep.Total.NRI),
		strconv.Itoa(rep.Total.OCI),
		strconv.Itoa(rep.Total.FN),
		strconv.Itoa(rep.Total.HeldCount),
		strconv.Itoa(rep.Total.Variance),
	})

	return ExportCSV(headers, rows)
}

// ConvertCategoryReportToCSV converts CategoryReportResult to sanitized CSV bytes
func ConvertCategoryReportToCSV(rep *CategoryReportResult) ([]byte, error) {
	headers := []string{"Institution ID", "Institution Name", "Program", "Academic Year", "Category", "Enrolled Count"}
	var rows [][]string
	for _, r := range rep.Rows {
		rows = append(rows, []string{
			r.InstitutionID,
			r.InstitutionName,
			r.ProgramID,
			r.AcademicYear,
			r.Category,
			strconv.Itoa(r.EnrolledCount),
		})
	}
	return ExportCSV(headers, rows)
}

// ConvertFlaggedReportToCSV converts FlaggedReportResult to sanitized CSV bytes
func ConvertFlaggedReportToCSV(rep *FlaggedReportResult) ([]byte, error) {
	headers := []string{"Institution ID", "Institution Name", "Rule Code", "Severity", "Review Status", "Flag Count"}
	var rows [][]string
	for _, r := range rep.Rows {
		rows = append(rows, []string{
			r.InstitutionID,
			r.InstitutionName,
			r.RuleCode,
			r.Severity,
			r.ReviewStatus,
			strconv.Itoa(r.Count),
		})
	}
	return ExportCSV(headers, rows)
}
