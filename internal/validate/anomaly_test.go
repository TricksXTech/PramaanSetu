package validate_test

import (
	"testing"

	"apaar-verify/internal/contracts"
	"apaar-verify/internal/validate"
)

func TestEvaluateAnomaly_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		history        []int
		observed       int
		expectedStatus contracts.AnomalyStatus
	}{
		{
			name:           "Insufficient history (0 years)",
			history:        []int{},
			observed:       120,
			expectedStatus: contracts.AnomalyStatusInsufficientHistory,
		},
		{
			name:           "Insufficient history (1 year)",
			history:        []int{500},
			observed:       520,
			expectedStatus: contracts.AnomalyStatusInsufficientHistory,
		},
		{
			name:           "Normal enrolment consistent with baseline (3 years)",
			history:        []int{480, 500, 520},
			observed:       510,
			expectedStatus: contracts.AnomalyStatusOK,
		},
		{
			name:           "Extreme enrolment spike (> 2 stddev / jump)",
			history:        []int{500, 510, 490},
			observed:       1200,
			expectedStatus: contracts.AnomalyStatusWarning,
		},
		{
			name:           "Extreme enrolment collapse",
			history:        []int{500, 520, 510},
			observed:       50,
			expectedStatus: contracts.AnomalyStatusWarning,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := validate.EvaluateAnomaly("INST-001", tc.history, tc.observed, 2)
			if res.Status != tc.expectedStatus {
				t.Errorf("expected status %s, got %s (observed: %d, mean: %.1f, stddev: %.1f)",
					tc.expectedStatus, res.Status, tc.observed, res.BaselineMean, res.BaselineStdDev)
			}
		})
	}
}

