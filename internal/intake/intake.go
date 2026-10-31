package intake

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"apaar-verify/internal/contracts"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNoApprovedIntake = errors.New("no approved intake found for institution, program and academic year")
	ErrIntakeExceeded   = errors.New("regular intake limit exceeded; row held under documented hold policy")
	ErrAllowanceExceded = errors.New("supernumerary category allowance cap exceeded; row held under documented hold policy")
)

// Reconciler handles serialized intake verification inside database transactions
type Reconciler struct {
	mu sync.Mutex // For in-memory concurrency testing with -race
}

func NewReconciler() *Reconciler {
	return &Reconciler{}
}

// ReconcileSeat verifies and claims an intake seat inside an active transaction.
// Non-negotiable Rule #9: Intake reconciliation is serialized per institution/program/year
// by locking the approved_intake row inside the transaction.
func (r *Reconciler) ReconcileSeat(
	ctx context.Context,
	tx pgx.Tx,
	institutionID, programID, academicYear string,
	row *contracts.CSVRow,
) (*contracts.IntakeResult, *contracts.Flag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. Lock the approved_intake row with SELECT ... FOR UPDATE
	lockQuery := `
		SELECT regular_limit, applicable_allowances, policy_version
		FROM approved_intake
		WHERE institution_id = $1 AND program_id = $2 AND academic_year = $3
		FOR UPDATE
	`

	var regularLimit, allowances int
	var policyVersion string

	err := tx.QueryRow(ctx, lockQuery, institutionID, programID, academicYear).Scan(
		&regularLimit,
		&allowances,
		&policyVersion,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, fmt.Errorf("%w: %s/%s/%s", ErrNoApprovedIntake, institutionID, programID, academicYear)
		}
		return nil, nil, err
	}

	// 2. Count existing accepted regular admissions
	countRegularQuery := `
		SELECT COUNT(*)
		FROM enrolments
		WHERE institution_id = $1 AND program_id = $2 AND academic_year = $3
		  AND is_supernumerary = false
		  AND is_nri = false
		  AND oci_status = false
		  AND is_foreign_national = false
	`
	var acceptedRegular int
	if err := tx.QueryRow(ctx, countRegularQuery, institutionID, programID, academicYear).Scan(&acceptedRegular); err != nil {
		return nil, nil, err
	}

	// 3. Count existing accepted supernumerary admissions
	countSuperQuery := `
		SELECT COUNT(*)
		FROM enrolments
		WHERE institution_id = $1 AND program_id = $2 AND academic_year = $3
		  AND (is_supernumerary = true OR is_nri = true OR oci_status = true OR is_foreign_national = true)
	`
	var acceptedSuper int
	if err := tx.QueryRow(ctx, countSuperQuery, institutionID, programID, academicYear).Scan(&acceptedSuper); err != nil {
		return nil, nil, err
	}

	isSuper := row.IsSupernumerary || row.IsNRI || row.OCIStatus || row.IsForeignNational

	res := &contracts.IntakeResult{
		RegularLimit:          regularLimit,
		Allowances:            allowances,
		AcceptedRegular:       acceptedRegular,
		AcceptedSupernumerary: acceptedSuper,
	}

	// 4. Evaluate seat capacity
	if isSuper {
		// Evaluated against allowances, never consumes regular seats
		if acceptedSuper >= allowances {
			res.OverflowSupernumerary = true
			flag := &contracts.Flag{
				RuleCode:      "V5",
				Severity:      "warning",
				Reason:        fmt.Sprintf("Supernumerary/NRI/FN allowance cap (%d) reached. Row held under documented hold policy.", allowances),
				Evidence:      map[string]any{"allowance_limit": allowances, "current_supernumerary": acceptedSuper},
				TargetType:    contracts.TargetUploadRow,
				PolicyVersion: policyVersion,
			}
			return res, flag, nil
		}
	} else {
		// Evaluated against regular_limit
		if acceptedRegular >= regularLimit {
			res.OverflowRegular = true
			flag := &contracts.Flag{
				RuleCode:      "V5",
				Severity:      "warning",
				Reason:        fmt.Sprintf("Regular intake limit (%d) reached for %s. Row held under documented hold policy.", regularLimit, programID),
				Evidence:      map[string]any{"regular_limit": regularLimit, "current_regular": acceptedRegular},
				TargetType:    contracts.TargetUploadRow,
				PolicyVersion: policyVersion,
			}
			return res, flag, nil
		}
	}

	return res, nil, nil
}
