package validate

import (
	"context"
	"fmt"
	"math"

	"apaar-verify/internal/contracts"
	"github.com/jackc/pgx/v5"
)

// EvaluateAnomaly performs pure statistical analysis of historical enrolment data (Rule V9)
// If years of history < minYears (default 2), returns AnomalyStatusInsufficientHistory
// If observed deviates by > 2 standard deviations from baseline mean, returns AnomalyStatusWarning
func EvaluateAnomaly(
	institutionID string,
	historicalCounts []int,
	observed int,
	minYears int,
) contracts.AnomalyResult {
	if minYears <= 0 {
		minYears = 2
	}

	result := contracts.AnomalyResult{
		InstitutionID:  institutionID,
		Metric:         "total_enrolment_volume",
		Observed:       float64(observed),
		YearsOfHistory: len(historicalCounts),
	}

	if len(historicalCounts) < minYears {
		result.Status = contracts.AnomalyStatusInsufficientHistory
		return result
	}

	// Compute Mean
	var sum float64
	for _, c := range historicalCounts {
		sum += float64(c)
	}
	mean := sum / float64(len(historicalCounts))
	result.BaselineMean = mean

	// Compute Standard Deviation
	var varianceSum float64
	for _, c := range historicalCounts {
		diff := float64(c) - mean
		varianceSum += diff * diff
	}
	stdDev := math.Sqrt(varianceSum / float64(len(historicalCounts)))
	result.BaselineStdDev = stdDev

	// Threshold: 2 standard deviations, or at least 40% deviation when stdDev is very tight
	effectiveThreshold := 2.0 * stdDev
	minThreshold := mean * 0.40
	if effectiveThreshold < minThreshold {
		effectiveThreshold = minThreshold
	}

	delta := math.Abs(float64(observed) - mean)
	if delta > effectiveThreshold && observed > 0 {
		result.Status = contracts.AnomalyStatusWarning
	} else {
		result.Status = contracts.AnomalyStatusOK
	}

	return result
}

// DetectInstitutionAnomaly evaluates historical anomaly from the database (Rule V9)
func DetectInstitutionAnomaly(
	ctx context.Context,
	tx pgx.Tx,
	institutionID string,
	currentYear string,
	observedTotal int,
	policyVersion string,
) (contracts.AnomalyResult, *contracts.Flag, error) {
	if policyVersion == "" {
		policyVersion = "AICTE-2024-V1"
	}

	// Query historical years from institution_history
	query := `
		SELECT total_enrolment
		FROM institution_history
		WHERE institution_id = $1 AND academic_year != $2
		ORDER BY academic_year ASC
	`
	rows, err := tx.Query(ctx, query, institutionID, currentYear)
	if err != nil {
		return contracts.AnomalyResult{
			InstitutionID: institutionID,
			Status:        contracts.AnomalyStatusInsufficientHistory,
		}, nil, err
	}
	defer rows.Close()

	var history []int
	for rows.Next() {
		var count int
		if err := rows.Scan(&count); err == nil {
			history = append(history, count)
		}
	}

	result := EvaluateAnomaly(institutionID, history, observedTotal, 2)

	var flag *contracts.Flag
	if result.Status == contracts.AnomalyStatusWarning {
		flag = &contracts.Flag{
			RuleCode: "V9",
			Severity: "warning",
			Reason: fmt.Sprintf("Institution enrolment volume anomaly: observed %d vs baseline mean %.1f (stddev %.1f)",
				observedTotal, result.BaselineMean, result.BaselineStdDev),
			Evidence: map[string]any{
				"observed":         result.Observed,
				"baseline_mean":    result.BaselineMean,
				"baseline_stddev":  result.BaselineStdDev,
				"years_of_history": result.YearsOfHistory,
				"status":           string(result.Status),
			},
			TargetType:    contracts.TargetInstitution,
			TargetID:      institutionID,
			PolicyVersion: policyVersion,
		}
	}

	return result, flag, nil
}

