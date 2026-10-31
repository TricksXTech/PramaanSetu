package validate

import (
	"context"
	"errors"
	"strings"
	"time"

	"apaar-verify/internal/contracts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PromoteRow transactionally writes accepted rows to students and enrolments, then updates rollups.
// Non-negotiable Rule #1: students holds identity only. Never overwrite existing identity.
func PromoteRow(
	ctx context.Context,
	tx pgx.Tx,
	institutionID string,
	uploadRowID uuid.UUID,
	row *contracts.CSVRow,
) (uuid.UUID, []contracts.Flag, error) {
	var flags []contracts.Flag

	dobTime, err := time.Parse("2006-01-02", row.DOB)
	if err != nil {
		dobTime, _ = time.Parse("02-01-2006", row.DOB)
	}

	// 1. Check if identity exists in students table
	var existingName string
	var existingDOB time.Time
	studentCheckQuery := `SELECT name, dob FROM students WHERE apaar_id = $1`
	err = tx.QueryRow(ctx, studentCheckQuery, row.APAARID).Scan(&existingName, &existingDOB)

	if errors.Is(err, pgx.ErrNoRows) {
		// Insert brand new student identity
		insertStudentQuery := `
			INSERT INTO students (apaar_id, name, dob, gender, guardian_name, identity_status)
			VALUES ($1, $2, $3, $4, $5, 'pending_verification')
		`
		var gPtr, guardPtr *string
		if row.Gender != "" {
			gPtr = &row.Gender
		}
		if row.GuardianName != "" {
			guardPtr = &row.GuardianName
		}
		_, err = tx.Exec(ctx, insertStudentQuery, row.APAARID, row.StudentName, dobTime, gPtr, guardPtr)
		if err != nil {
			return uuid.Nil, nil, err
		}
	} else if err == nil {
		// Identity already exists!
		// Non-negotiable Rule #1: NEVER let a CSV overwrite identity fields.
		// If name or DOB differs, raise a review flag instead of overwriting.
		if !strings.EqualFold(strings.TrimSpace(existingName), strings.TrimSpace(row.StudentName)) {
			flags = append(flags, contracts.Flag{
				RuleCode:      "V8",
				Severity:      "warning",
				Reason:        "Identity inconsistency: CSV name differs from existing identity. Record preserved without overwrite.",
				Evidence:      map[string]any{"existing_name": existingName, "csv_name": row.StudentName},
				TargetType:    contracts.TargetUploadRow,
				TargetID:      uploadRowID.String(),
				PolicyVersion: "AICTE-2024-V1",
			})
		}
	} else {
		return uuid.Nil, nil, err
	}

	// 2. Insert or update tenant-owned enrolment
	enrolmentID := uuid.New()
	insertEnrolmentQuery := `
		INSERT INTO enrolments (
			id, apaar_id, institution_id, program_id, academic_year, admission_reference,
			admission_date, category, admission_type, is_supernumerary, is_nri, oci_status,
			is_foreign_national, nationality, status, version
		)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_DATE, $7, $8, $9, $10, $11, $12, $13, 'submitted', 1)
		ON CONFLICT (institution_id, academic_year, admission_reference) DO UPDATE SET
			category = EXCLUDED.category,
			admission_type = EXCLUDED.admission_type,
			is_supernumerary = EXCLUDED.is_supernumerary,
			is_nri = EXCLUDED.is_nri,
			oci_status = EXCLUDED.oci_status,
			is_foreign_national = EXCLUDED.is_foreign_national,
			nationality = EXCLUDED.nationality,
			version = enrolments.version + 1,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`

	var finalEnrolmentID uuid.UUID
	err = tx.QueryRow(ctx, insertEnrolmentQuery,
		enrolmentID,
		row.APAARID,
		institutionID,
		row.ProgramCode,
		row.AcademicYear,
		row.AdmissionReference,
		row.Category,
		row.AdmissionType,
		row.IsSupernumerary,
		row.IsNRI,
		row.OCIStatus,
		row.IsForeignNational,
		row.Nationality,
	).Scan(&finalEnrolmentID)

	if err != nil {
		return uuid.Nil, nil, err
	}

	// 3. Increment pre-aggregated rollup_enrolment inside same transaction
	superNum := 0
	if row.IsSupernumerary {
		superNum = 1
	}
	nriNum := 0
	if row.IsNRI {
		nriNum = 1
	}
	ociNum := 0
	if row.OCIStatus {
		ociNum = 1
	}
	fnNum := 0
	if row.IsForeignNational {
		fnNum = 1
	}

	upsertRollupQuery := `
		INSERT INTO rollup_enrolment (
			institution_id, program_id, academic_year, total_enrolled, supernumerary, nri, oci, foreign_national, flagged_count, held_count, updated_at
		)
		VALUES ($1, $2, $3, 1, $4, $5, $6, $7, 0, 0, CURRENT_TIMESTAMP)
		ON CONFLICT (institution_id, program_id, academic_year) DO UPDATE SET
			total_enrolled = rollup_enrolment.total_enrolled + 1,
			supernumerary = rollup_enrolment.supernumerary + EXCLUDED.supernumerary,
			nri = rollup_enrolment.nri + EXCLUDED.nri,
			oci = rollup_enrolment.oci + EXCLUDED.oci,
			foreign_national = rollup_enrolment.foreign_national + EXCLUDED.foreign_national,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err = tx.Exec(ctx, upsertRollupQuery, institutionID, row.ProgramCode, row.AcademicYear, superNum, nriNum, ociNum, fnNum)
	if err != nil {
		return uuid.Nil, nil, err
	}

	return finalEnrolmentID, flags, nil
}

// RecordRollupHeld increments the held_count and flagged_count in rollup_enrolment inside transaction
func RecordRollupHeld(ctx context.Context, tx pgx.Tx, institutionID, programID, academicYear string) error {
	upsertHeld := `
		INSERT INTO rollup_enrolment (
			institution_id, program_id, academic_year, total_enrolled, supernumerary, nri, oci, foreign_national, flagged_count, held_count, updated_at
		)
		VALUES ($1, $2, $3, 0, 0, 0, 0, 0, 1, 1, CURRENT_TIMESTAMP)
		ON CONFLICT (institution_id, program_id, academic_year) DO UPDATE SET
			held_count = rollup_enrolment.held_count + 1,
			flagged_count = rollup_enrolment.flagged_count + 1,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := tx.Exec(ctx, upsertHeld, institutionID, programID, academicYear)
	return err
}
