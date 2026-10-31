package validate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"apaar-verify/internal/contracts"
	"apaar-verify/internal/intake"
	"apaar-verify/internal/match"
	"github.com/jackc/pgx/v5"
)

type Engine struct {
	registry   contracts.RegistryAdapter
	reconciler *intake.Reconciler
	matcher    *match.Matcher
}

func NewEngine(registry contracts.RegistryAdapter) *Engine {
	if registry == nil {
		registry = NewMockRegistryAdapter()
	}
	return &Engine{
		registry:   registry,
		reconciler: intake.NewReconciler(),
		matcher:    match.NewMatcher(contracts.DefaultMatchConfig()),
	}
}

// Validate executes rules V1-V8 sequentially in table-driven order
func (e *Engine) Validate(
	ctx context.Context,
	tx pgx.Tx,
	row *contracts.CSVRow,
	meta *contracts.ValidationMetadata,
) ([]contracts.Flag, contracts.RowOutcome, error) {
	var flags []contracts.Flag

	// 1. Rule V1: APAAR syntax check (exactly 12 ASCII digits)
	v1Flags, v1Outcome := ValidateV1(row, meta)
	if len(v1Flags) > 0 {
		return v1Flags, v1Outcome, nil
	}

	// 2. Rule V2: APAAR Registry check (mock adapter)
	v2Flags, v2Outcome, err := e.ValidateV2(ctx, row, meta)
	if err != nil {
		return nil, contracts.RowOutcomePendingRegistry, err
	}
	if len(v2Flags) > 0 {
		flags = append(flags, v2Flags...)
		if v2Outcome != contracts.RowOutcomeAccepted {
			return flags, v2Outcome, nil
		}
	}

	// 3. Rule V3: Duplicate within same file
	v3Flags, v3Outcome := ValidateV3(row, meta)
	if len(v3Flags) > 0 {
		flags = append(flags, v3Flags...)
		if v3Outcome != contracts.RowOutcomeAccepted {
			return flags, v3Outcome, nil
		}
	}

	// 4. Rule V6: Policy Consistency Checks (dynamic PolicyConfig)
	v6Flags, v6Outcome := ValidateV6(row, meta)
	if len(v6Flags) > 0 {
		flags = append(flags, v6Flags...)
		if v6Outcome != contracts.RowOutcomeAccepted {
			return flags, v6Outcome, nil
		}
	}

	// 5. Rule V4: Existing enrolment in database for this institution/year/ref
	if tx != nil && meta != nil {
		v4Flags, v4Outcome, err := ValidateV4(ctx, tx, row, meta)
		if err != nil {
			return nil, contracts.RowOutcomeFlaggedHeld, err
		}
		if len(v4Flags) > 0 || v4Outcome != contracts.RowOutcomeAccepted {
			flags = append(flags, v4Flags...)
			return flags, v4Outcome, nil
		}

		// 6. Rule V7: Cross-Institution Concurrent Enrolment (Rule #4: Review lead, never auto-reject)
		v7Flags, err := ValidateV7(ctx, tx, row, meta)
		if err == nil && len(v7Flags) > 0 {
			flags = append(flags, v7Flags...)
		}

		// 7. Rule V8: Bounded Fuzzy Candidate Matching (pg_trgm + DOB candidate blocking)
		v8Flags, err := e.ValidateV8(ctx, tx, row, meta)
		if err == nil && len(v8Flags) > 0 {
			flags = append(flags, v8Flags...)
		}

		// 8. Rule V5: Intake Reconciliation & Cap Enforcement
		// Non-negotiable Rule #9: Serialized per institution/program/year using a locked intake row
		intakeRes, v5Flag, err := e.reconciler.ReconcileSeat(ctx, tx, meta.InstitutionID, row.ProgramCode, row.AcademicYear, row)
		if err == nil && v5Flag != nil {
			v5Flag.TargetID = meta.UploadRowID.String()
			flags = append(flags, *v5Flag)
			if intakeRes.OverflowRegular || intakeRes.OverflowSupernumerary {
				// Held under documented hold policy; not promoted
				return flags, contracts.RowOutcomeFlaggedHeld, nil
			}
		}
	}

	return flags, contracts.RowOutcomeAccepted, nil
}

// ValidateV1 checks that APAAR ID is exactly 12 ASCII digits
func ValidateV1(row *contracts.CSVRow, meta *contracts.ValidationMetadata) ([]contracts.Flag, contracts.RowOutcome) {
	targetID := ""
	polVer := "AICTE-2024-V1"
	if meta != nil {
		targetID = meta.UploadRowID.String()
		if meta.PolicyConfig != nil {
			polVer = meta.PolicyConfig.PolicyVersion
		}
	}

	apaar := row.APAARID
	if len(apaar) != 12 || !isAllDigitsASCII(apaar) {
		return []contracts.Flag{
			{
				RuleCode:      "V1",
				Severity:      "error",
				Reason:        "Missing or malformed APAAR: must be exactly 12 ASCII digits",
				Evidence:      map[string]any{"submitted_length": len(apaar), "is_ascii_digits": isAllDigitsASCII(apaar)},
				TargetType:    contracts.TargetUploadRow,
				TargetID:      targetID,
				PolicyVersion: polVer,
			},
		}, contracts.RowOutcomeRejectedInvalid
	}

	return nil, contracts.RowOutcomeAccepted
}

// ValidateV2 queries the registry adapter
func (e *Engine) ValidateV2(ctx context.Context, row *contracts.CSVRow, meta *contracts.ValidationMetadata) ([]contracts.Flag, contracts.RowOutcome, error) {
	targetID := ""
	polVer := "AICTE-2024-V1"
	if meta != nil {
		targetID = meta.UploadRowID.String()
		if meta.PolicyConfig != nil {
			polVer = meta.PolicyConfig.PolicyVersion
		}
	}

	res, err := e.registry.Lookup(ctx, row.APAARID)
	if err != nil {
		return nil, contracts.RowOutcomePendingRegistry, err
	}

	switch res.Status {
	case contracts.RegistryStatusUnavailable:
		// Non-negotiable Rule #6: "Unavailable is pending/retry, not invalid"
		return []contracts.Flag{
			{
				RuleCode:      "V2",
				Severity:      "info",
				Reason:        "APAAR registry lookup temporarily unavailable; held in retry queue",
				Evidence:      map[string]any{"provenance": res.Provenance, "registry_status": "unavailable"},
				TargetType:    contracts.TargetUploadRow,
				TargetID:      targetID,
				PolicyVersion: polVer,
			},
		}, contracts.RowOutcomePendingRegistry, nil

	case contracts.RegistryStatusNotFound:
		return []contracts.Flag{
			{
				RuleCode:      "V2",
				Severity:      "warning",
				Reason:        "APAAR ID not found in registry (mock adapter)",
				Evidence:      map[string]any{"provenance": res.Provenance, "registry_status": "not_found"},
				TargetType:    contracts.TargetUploadRow,
				TargetID:      targetID,
				PolicyVersion: polVer,
			},
		}, contracts.RowOutcomeFlaggedHeld, nil

	case contracts.RegistryStatusIdentityMismatch:
		return []contracts.Flag{
			{
				RuleCode:      "V2",
				Severity:      "conflict",
				Reason:        "Student name or DOB does not match registry record (mock adapter)",
				Evidence:      map[string]any{"provenance": res.Provenance, "registry_status": "identity_mismatch"},
				TargetType:    contracts.TargetUploadRow,
				TargetID:      targetID,
				PolicyVersion: polVer,
			},
		}, contracts.RowOutcomeFlaggedHeld, nil

	case contracts.RegistryStatusMatched:
		return nil, contracts.RowOutcomeAccepted, nil

	default:
		return nil, contracts.RowOutcomeAccepted, nil
	}
}

// ValidateV3 detects repeated submissions or multiple enrolments within the same file
func ValidateV3(row *contracts.CSVRow, meta *contracts.ValidationMetadata) ([]contracts.Flag, contracts.RowOutcome) {
	if meta == nil || meta.SeenInFile == nil {
		return nil, contracts.RowOutcomeAccepted
	}

	targetID := meta.UploadRowID.String()
	polVer := "AICTE-2024-V1"
	if meta.PolicyConfig != nil {
		polVer = meta.PolicyConfig.PolicyVersion
	}

	// Exact submission signature in same file: apaar_id + program + ref
	exactKey := fmt.Sprintf("%s|%s|%s", row.APAARID, row.ProgramCode, row.AdmissionReference)
	if prevRow, exists := meta.SeenInFile[exactKey]; exists {
		return []contracts.Flag{
			{
				RuleCode:      "V3",
				Severity:      "warning",
				Reason:        fmt.Sprintf("Repeated submission within same file (identical to row %d)", prevRow),
				Evidence:      map[string]any{"duplicate_of_row": prevRow, "type": "exact_file_duplicate"},
				TargetType:    contracts.TargetUploadRow,
				TargetID:      targetID,
				PolicyVersion: polVer,
			},
		}, contracts.RowOutcomeFlaggedHeld
	}

	meta.SeenInFile[exactKey] = meta.RowNumber
	return nil, contracts.RowOutcomeAccepted
}

// ValidateV4 checks database for existing enrolment with same institution, academic_year, and admission_reference
func ValidateV4(ctx context.Context, tx pgx.Tx, row *contracts.CSVRow, meta *contracts.ValidationMetadata) ([]contracts.Flag, contracts.RowOutcome, error) {
	query := `
		SELECT id, apaar_id, program_id, category, admission_type,
		       is_supernumerary, is_nri, oci_status, is_foreign_national, nationality, version
		FROM enrolments
		WHERE institution_id = $1 AND academic_year = $2 AND admission_reference = $3
	`

	var existingID string
	var existingAPAAR, existingProgram, existingCat, existingAdmType, existingNat string
	var existingSuper, existingNRI, existingOCI, existingFN bool
	var existingVersion int

	err := tx.QueryRow(ctx, query, meta.InstitutionID, row.AcademicYear, row.AdmissionReference).Scan(
		&existingID,
		&existingAPAAR,
		&existingProgram,
		&existingCat,
		&existingAdmType,
		&existingSuper,
		&existingNRI,
		&existingOCI,
		&existingFN,
		&existingNat,
		&existingVersion,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, contracts.RowOutcomeAccepted, nil
		}
		return nil, contracts.RowOutcomeFlaggedHeld, err
	}

	// Check identical re-upload
	isIdentical := existingAPAAR == row.APAARID &&
		existingProgram == row.ProgramCode &&
		existingCat == row.Category &&
		existingAdmType == row.AdmissionType &&
		existingSuper == row.IsSupernumerary &&
		existingNRI == row.IsNRI &&
		existingOCI == row.OCIStatus &&
		existingFN == row.IsForeignNational &&
		strings.EqualFold(existingNat, row.Nationality)

	if isIdentical {
		return nil, contracts.RowOutcomeNoOpIdentical, nil
	}

	polVer := "AICTE-2024-V1"
	if meta.PolicyConfig != nil {
		polVer = meta.PolicyConfig.PolicyVersion
	}

	// If APAAR differs for the same stable tenant admission reference -> Conflict / Stale!
	if existingAPAAR != row.APAARID {
		return []contracts.Flag{
			{
				RuleCode:      "V4",
				Severity:      "conflict",
				Reason:        "Existing enrolment with same admission reference belongs to different APAAR ID",
				Evidence:      map[string]any{"existing_enrolment_id": existingID, "conflict": "apaar_id_mismatch"},
				TargetType:    contracts.TargetEnrolment,
				TargetID:      existingID,
				PolicyVersion: polVer,
			},
		}, contracts.RowOutcomeConflictStale, nil
	}

	return nil, contracts.RowOutcomeAccepted, nil
}

func isAllDigitsASCII(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// ValidateV7 checks for concurrent enrolment of the same APAAR ID across different institutions
// Non-negotiable Rule #4: Review leads, never fraud verdicts
func ValidateV7(ctx context.Context, tx pgx.Tx, row *contracts.CSVRow, meta *contracts.ValidationMetadata) ([]contracts.Flag, error) {
	if tx == nil || meta == nil || row.APAARID == "" {
		return nil, nil
	}

	targetID := meta.UploadRowID.String()
	polVer := "AICTE-2024-V1"
	if meta.PolicyConfig != nil {
		polVer = meta.PolicyConfig.PolicyVersion
	}

	query := `
		SELECT e.id, e.institution_id, COALESCE(i.name, e.institution_id), e.program_id, e.academic_year
		FROM enrolments e
		LEFT JOIN institutions i ON e.institution_id = i.id
		WHERE e.apaar_id = $1 AND e.institution_id != $2 AND e.academic_year = $3 AND e.status IN ('submitted', 'attested')
		LIMIT 1
	`

	var otherEnrolID, otherInstID, otherInstName, otherProg, otherYear string
	err := tx.QueryRow(ctx, query, row.APAARID, meta.InstitutionID, row.AcademicYear).Scan(
		&otherEnrolID, &otherInstID, &otherInstName, &otherProg, &otherYear,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	evidenceMap := map[string]any{
		"primary_institution_id": meta.InstitutionID,
		"primary_program_code":   row.ProgramCode,
		"primary_academic_year":  row.AcademicYear,
		"other_institution_id":   otherInstID,
		"other_institution_name": otherInstName,
		"other_program_code":     otherProg,
		"other_academic_year":    otherYear,
		"masked_apaar":           contracts.MaskAPAAR(row.APAARID),
		"masked_dob":             contracts.MaskDOB(row.DOB),
		"other_enrolment_id":     otherEnrolID,
	}

	return []contracts.Flag{
		{
			RuleCode:      "V7",
			Severity:      "warning",
			Reason:        fmt.Sprintf("Concurrent enrolment across institutions detected with %s (%s)", otherInstName, otherProg),
			Evidence:      evidenceMap,
			TargetType:    contracts.TargetUploadRow,
			TargetID:      targetID,
			PolicyVersion: polVer,
		},
	}, nil
}

// ValidateV8 performs bounded fuzzy identity candidate matching using pg_trgm + DOB blocking (Rule #4)
func (e *Engine) ValidateV8(ctx context.Context, tx pgx.Tx, row *contracts.CSVRow, meta *contracts.ValidationMetadata) ([]contracts.Flag, error) {
	if tx == nil || meta == nil || e.matcher == nil {
		return nil, nil
	}

	candidates, err := e.matcher.FindCandidates(ctx, tx, row)
	if err != nil || len(candidates) == 0 {
		return nil, err
	}

	targetID := meta.UploadRowID.String()
	polVer := "AICTE-2024-V1"
	if meta.PolicyConfig != nil {
		polVer = meta.PolicyConfig.PolicyVersion
	}

	candList := make([]map[string]any, 0, len(candidates))
	for _, c := range candidates {
		candList = append(candList, map[string]any{
			"candidate_apaar_masked": contracts.MaskAPAAR(c.CandidateEnrolmentRef),
			"score":                  fmt.Sprintf("%.2f", c.Score),
			"matched_fields":         c.MatchedFields,
			"blocking_key":           c.BlockingKey,
			"method":                 c.Method,
		})
	}

	return []contracts.Flag{
		{
			RuleCode: "V8",
			Severity: "warning",
			Reason:   fmt.Sprintf("Found %d potential duplicate identity candidate(s) via bounded fuzzy matching", len(candidates)),
			Evidence: map[string]any{
				"candidate_count": len(candidates),
				"candidates":      candList,
				"masked_apaar":    contracts.MaskAPAAR(row.APAARID),
				"masked_dob":      contracts.MaskDOB(row.DOB),
			},
			TargetType:    contracts.TargetUploadRow,
			TargetID:      targetID,
			PolicyVersion: polVer,
		},
	}, nil
}
