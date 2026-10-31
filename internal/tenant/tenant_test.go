package tenant_test

import (
	"testing"

	"apaar-verify/internal/models"
	"apaar-verify/internal/tenant"
	"github.com/google/uuid"
)

func TestEnforceInstitution_Isolation(t *testing.T) {
	inst1 := "INST-001"
	inst2 := "INST-002"

	// 1. Institution Admin 1 accessing own institution -> SUCCESS
	admin1 := &tenant.Scope{
		UserID:        uuid.New(),
		Role:          models.RoleInstitutionAdmin,
		InstitutionID: inst1,
	}
	if err := admin1.EnforceInstitution(inst1); err != nil {
		t.Fatalf("expected admin1 to access inst1, got error: %v", err)
	}

	// 2. Institution Admin 1 accessing institution 2 -> MUST FAIL with ErrCrossTenantAccessDenied
	err := admin1.EnforceInstitution(inst2)
	if err == nil {
		t.Fatalf("expected cross-tenant access denial when admin1 accesses inst2, but succeeded")
	}

	// 3. AICTE Admin can supervise institutions
	aicteAdmin := &tenant.Scope{
		UserID: uuid.New(),
		Role:   models.RoleAICTEAdmin,
	}
	if err := aicteAdmin.EnforceInstitution(inst1); err != nil {
		t.Fatalf("expected aicteAdmin to access inst1, got error: %v", err)
	}
	if err := aicteAdmin.EnforceInstitution(inst2); err != nil {
		t.Fatalf("expected aicteAdmin to access inst2, got error: %v", err)
	}

	// 4. Student or Verifier accessing institution endpoints -> MUST FAIL
	student := &tenant.Scope{
		UserID:  uuid.New(),
		Role:    models.RoleStudent,
		APAARID: "123456789012",
	}
	if err := student.EnforceInstitution(inst1); err == nil {
		t.Fatalf("expected student to be denied institution-level management access")
	}
}

func TestEnforceStudent_Isolation(t *testing.T) {
	student1APAAR := "111122223333"
	student2APAAR := "444455556666"

	s1 := &tenant.Scope{
		UserID:  uuid.New(),
		Role:    models.RoleStudent,
		APAARID: student1APAAR,
	}

	// Student 1 accessing own records -> SUCCESS
	if err := s1.EnforceStudent(student1APAAR); err != nil {
		t.Fatalf("expected s1 to access own records, got error: %v", err)
	}

	// Student 1 accessing Student 2 records -> MUST FAIL
	if err := s1.EnforceStudent(student2APAAR); err == nil {
		t.Fatalf("expected student isolation failure when accessing another student's record")
	}
}

