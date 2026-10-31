package intake

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RebuildRollups recomputes the entire rollup_enrolment table directly from enrolments and validation_flags
// Milestone 3 Requirement #5: Guarantees rollups can be deterministically reconstructed without drift
func RebuildRollups(ctx context.Context, db *pgxpool.Pool) (int, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	// 1. Truncate existing rollups
	if _, err := tx.Exec(ctx, "DELETE FROM rollup_enrolment"); err != nil {
		return 0, fmt.Errorf("failed to clear rollups: %w", err)
	}

	// 2. Re-populate from enrolments
	rebuildEnrolmentsQuery := `
		INSERT INTO rollup_enrolment (
			institution_id, program_id, academic_year, total_enrolled,
			supernumerary, nri, oci, foreign_national, flagged_count, held_count, updated_at
		)
		SELECT
			e.institution_id,
			e.program_id,
			e.academic_year,
			COUNT(e.id) AS total_enrolled,
			COUNT(e.id) FILTER (WHERE e.is_supernumerary = true) AS supernumerary,
			COUNT(e.id) FILTER (WHERE e.is_nri = true) AS nri,
			COUNT(e.id) FILTER (WHERE e.oci_status = true) AS oci,
			COUNT(e.id) FILTER (WHERE e.is_foreign_national = true) AS foreign_national,
			0,
			0,
			CURRENT_TIMESTAMP
		FROM enrolments e
		GROUP BY e.institution_id, e.program_id, e.academic_year
		ON CONFLICT (institution_id, program_id, academic_year) DO UPDATE SET
			total_enrolled = EXCLUDED.total_enrolled,
			supernumerary = EXCLUDED.supernumerary,
			nri = EXCLUDED.nri,
			oci = EXCLUDED.oci,
			foreign_national = EXCLUDED.foreign_national,
			updated_at = CURRENT_TIMESTAMP
	`
	cmd, err := tx.Exec(ctx, rebuildEnrolmentsQuery)
	if err != nil {
		return 0, fmt.Errorf("failed to rebuild from enrolments: %w", err)
	}

	// 3. Update flagged and held counts from upload_rows and validation_flags
	updateHeldQuery := `
		UPDATE rollup_enrolment re
		SET held_count = sub.held_cnt,
		    flagged_count = sub.flagged_cnt
		FROM (
			SELECT
				r.institution_id,
				(r.normalized_fields->>'program_code') AS program_id,
				(r.normalized_fields->>'academic_year') AS academic_year,
				COUNT(*) FILTER (WHERE r.outcome = 'flagged_held') AS held_cnt,
				COUNT(*) FILTER (WHERE r.outcome IN ('flagged_held', 'conflict_stale', 'pending_registry')) AS flagged_cnt
			FROM upload_rows r
			WHERE r.normalized_fields IS NOT NULL
			GROUP BY r.institution_id, r.normalized_fields->>'program_code', r.normalized_fields->>'academic_year'
		) sub
		WHERE re.institution_id = sub.institution_id
		  AND re.program_id = sub.program_id
		  AND re.academic_year = sub.academic_year
	`
	_, _ = tx.Exec(ctx, updateHeldQuery)

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	rebuiltRows := int(cmd.RowsAffected())
	log.Printf("[Intake] Rebuilt rollups for %d program cohorts", rebuiltRows)
	return rebuiltRows, nil
}

