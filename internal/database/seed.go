package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"apaar-verify/internal/auth"
	"apaar-verify/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunSeed populates deterministic baseline records into the database with idempotent upserts
func RunSeed(ctx context.Context, db *pgxpool.Pool) error {
	defaultPassword := "Password@123"
	passwordHash, err := auth.HashPassword(defaultPassword)
	if err != nil {
		return fmt.Errorf("failed to hash default password: %w", err)
	}

	// 1. Seed Institutions
	log.Println("[Seed] Inserting institutions...")
	institutions := []models.Institution{
		{ID: "INST-AICTE-001", AICTEID: "AICTE-DL-001", Name: "Delhi Technological University (DTU)", State: "Delhi", Type: "State University"},
		{ID: "INST-AICTE-002", AICTEID: "AICTE-MH-002", Name: "College of Engineering Pune (COEP)", State: "Maharashtra", Type: "State Autonomous"},
	}

	for _, inst := range institutions {
		query := `
			INSERT INTO institutions (id, aicte_id, name, state, type)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				state = EXCLUDED.state,
				type = EXCLUDED.type
		`
		_, err := db.Exec(ctx, query, inst.ID, inst.AICTEID, inst.Name, inst.State, inst.Type)
		if err != nil {
			return fmt.Errorf("failed to insert institution %s: %w", inst.ID, err)
		}
	}

	// 2. Seed Programs
	log.Println("[Seed] Inserting programs...")
	programs := []models.Program{
		{ID: "BTECH-CSE", Name: "B.Tech Computer Science & Engineering", Department: "Computer Science", DurationYears: 4},
		{ID: "BTECH-ECE", Name: "B.Tech Electronics & Communication", Department: "Electronics", DurationYears: 4},
	}
	for _, prog := range programs {
		query := `
			INSERT INTO programs (id, name, department, duration_years)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				department = EXCLUDED.department
		`
		_, err := db.Exec(ctx, query, prog.ID, prog.Name, prog.Department, prog.DurationYears)
		if err != nil {
			return fmt.Errorf("failed to insert program %s: %w", prog.ID, err)
		}
	}

	// 3. Seed Approved Intake
	log.Println("[Seed] Inserting approved intake...")
	intakes := []models.ApprovedIntake{
		{
			InstitutionID:        "INST-AICTE-001",
			ProgramID:            "BTECH-CSE",
			AcademicYear:         "2024-25",
			RegularLimit:         2,
			ApplicableAllowances: 0,
			PolicyVersion:        "AICTE-2024-V1",
		},
		{
			InstitutionID:        "INST-AICTE-002",
			ProgramID:            "BTECH-CSE",
			AcademicYear:         "2024-25",
			RegularLimit:         60,
			ApplicableAllowances: 5,
			PolicyVersion:        "AICTE-2024-V1",
		},
	}
	for _, in := range intakes {
		query := `
			INSERT INTO approved_intake (institution_id, program_id, academic_year, regular_limit, applicable_allowances, policy_version)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (institution_id, program_id, academic_year) DO UPDATE SET
				regular_limit = EXCLUDED.regular_limit,
				applicable_allowances = EXCLUDED.applicable_allowances
		`
		_, err := db.Exec(ctx, query, in.InstitutionID, in.ProgramID, in.AcademicYear, in.RegularLimit, in.ApplicableAllowances, in.PolicyVersion)
		if err != nil {
			return fmt.Errorf("failed to insert approved intake: %w", err)
		}
	}

	// 4. Seed Users for all 4 roles
	log.Println("[Seed] Inserting users...")
	inst1 := "INST-AICTE-001"
	inst2 := "INST-AICTE-002"
	apaar1 := "123456789012"
	apaar2 := "987654321098"

	users := []models.User{
		{Email: "admin@aicte.gov.in", Role: models.RoleAICTEAdmin, PasswordHash: passwordHash},
		{Email: "admin@dtu.ac.in", Role: models.RoleInstitutionAdmin, InstitutionID: &inst1, PasswordHash: passwordHash},
		{Email: "admin@coep.ac.in", Role: models.RoleInstitutionAdmin, InstitutionID: &inst2, PasswordHash: passwordHash},
		{Email: "student1@example.com", Role: models.RoleStudent, APAARID: &apaar1, PasswordHash: passwordHash},
		{Email: "student2@example.com", Role: models.RoleStudent, APAARID: &apaar2, PasswordHash: passwordHash},
		{Email: "verifier@tcs.com", Role: models.RoleVerifier, PasswordHash: passwordHash},
	}

	for _, u := range users {
		query := `
			INSERT INTO users (email, password_hash, role, institution_id, apaar_id, status)
			VALUES ($1, $2, $3, $4, $5, 'active')
			ON CONFLICT (email) DO UPDATE SET
				password_hash = EXCLUDED.password_hash,
				role = EXCLUDED.role,
				institution_id = EXCLUDED.institution_id,
				apaar_id = EXCLUDED.apaar_id
		`
		_, err := db.Exec(ctx, query, u.Email, u.PasswordHash, u.Role, u.InstitutionID, u.APAARID)
		if err != nil {
			return fmt.Errorf("failed to insert user %s: %w", u.Email, err)
		}
	}

	// 5. Seed Students (Identity Only)
	log.Println("[Seed] Inserting student identities...")
	t1, _ := time.Parse("2006-01-02", "2004-03-12")
	t2, _ := time.Parse("2006-01-02", "2004-05-15")
	gM := "M"
	guard1 := "Ramesh Gupta"
	guard2 := "Rajesh Sharma"

	students := []models.Student{
		{APAARID: apaar1, Name: "Rohan Gupta", DOB: t1, Gender: &gM, GuardianName: &guard1, IdentityStatus: "verified"},
		{APAARID: apaar2, Name: "Aarav Sharma", DOB: t2, Gender: &gM, GuardianName: &guard2, IdentityStatus: "verified"},
	}

	for _, s := range students {
		query := `
			INSERT INTO students (apaar_id, name, dob, gender, guardian_name, identity_status)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (apaar_id) DO UPDATE SET
				name = EXCLUDED.name,
				dob = EXCLUDED.dob
		`
		_, err := db.Exec(ctx, query, s.APAARID, s.Name, s.DOB, s.Gender, s.GuardianName, s.IdentityStatus)
		if err != nil {
			return fmt.Errorf("failed to insert student %s: %w", s.APAARID, err)
		}
	}

	// 6. Pre-seed existing enrolment at COEP for cross-enrolment test
	log.Println("[Seed] Inserting pre-existing COEP enrolment for cross-institution test...")
	coepEnrolmentQuery := `
		INSERT INTO enrolments (
			id, apaar_id, institution_id, program_id, academic_year, admission_reference,
			admission_date, category, admission_type, is_supernumerary, is_nri, oci_status,
			is_foreign_national, nationality, status, version
		)
		VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5,
			$6, 'General', 'Regular', false, false, false,
			false, 'INDIAN', 'attested', 1
		)
		ON CONFLICT (institution_id, academic_year, admission_reference) DO NOTHING
	`
	_, err = db.Exec(ctx, coepEnrolmentQuery, apaar2, inst2, "BTECH-CSE", "2024-25", "COEP-2024-042", t2)
	if err != nil {
		return fmt.Errorf("failed to insert COEP enrolment: %w", err)
	}

	// 7. Seed API Key for DTU
	log.Println("[Seed] Inserting test API key for DTU...")
	rawAPIKey := "ak_live_dtu_test_token_123456789"
	keyHash := auth.HashToken(rawAPIKey)
	apiKeyQuery := `
		INSERT INTO api_keys (institution_id, key_hash, key_prefix, scopes)
		VALUES ($1, $2, 'ak_live_dtu', '{"upload:write", "upload:read"}')
		ON CONFLICT (key_hash) DO NOTHING
	`
	_, err = db.Exec(ctx, apiKeyQuery, inst1, keyHash)
	if err != nil {
		return fmt.Errorf("failed to insert API key: %w", err)
	}

	// 8. Seed Historical Enrolments for Rule V9 Anomaly Baselines
	log.Println("[Seed] Inserting historical institution enrolment data (Rule V9)...")
	historyRecords := []struct {
		instID string
		year   string
		total  int
	}{
		{inst1, "2021-22", 450},
		{inst1, "2022-23", 470},
		{inst1, "2023-24", 460},
		{inst2, "2021-22", 600},
		{inst2, "2022-23", 610},
		{inst2, "2023-24", 605},
	}
	for _, hr := range historyRecords {
		histQ := `
			INSERT INTO institution_history (institution_id, academic_year, total_enrolment)
			VALUES ($1, $2, $3)
			ON CONFLICT (institution_id, academic_year) DO UPDATE SET
				total_enrolment = EXCLUDED.total_enrolment
		`
		_, err := db.Exec(ctx, histQ, hr.instID, hr.year, hr.total)
		if err != nil {
			return fmt.Errorf("failed to insert institution history: %w", err)
		}
	}

	// 9. Pre-seed Sample V7 and V8 Review Leads for AICTE Review Workflow Demo
	log.Println("[Seed] Inserting sample review queue leads (V7 & V8)...")
	v7Evidence := `{"primary_institution_id":"INST-AICTE-001","primary_program_code":"BTECH-CSE","primary_academic_year":"2024-25","other_institution_id":"INST-AICTE-002","other_institution_name":"College of Engineering Pune (COEP)","other_program_code":"BTECH-CSE","other_academic_year":"2024-25","masked_apaar":"9876****1098","masked_dob":"2004-**-**"}`
	v8Evidence := `{"masked_apaar":"1234****9012","masked_dob":"2004-**-**","candidates":[{"candidate_apaar_masked":"9876****1098","score":"0.88","matched_fields":["dob","student_name"],"blocking_key":"dob:2004-05-15","method":"pg_trgm_similarity_with_dob_blocking"}]}`

	flagQ := `
		INSERT INTO validation_flags (institution_id, rule_code, severity, reason, evidence, policy_version, review_status)
		VALUES
			($1, 'V7', 'warning', 'Concurrent enrolment across institutions detected with College of Engineering Pune (COEP) (BTECH-CSE)', $2::jsonb, 'AICTE-2024-V1', 'pending'),
			($1, 'V8', 'warning', 'Found 1 potential duplicate identity candidate(s) via bounded fuzzy matching', $3::jsonb, 'AICTE-2024-V1', 'pending')
		ON CONFLICT DO NOTHING
	`
	_, err = db.Exec(ctx, flagQ, inst1, v7Evidence, v8Evidence)
	if err != nil {
		log.Printf("[Seed] Warning: sample flag insertion error: %v", err)
	}

	return nil
}
