package validate_test

import (
	"testing"

	"apaar-verify/internal/contracts"
)

func TestRuleV7_CrossInstitutionEvidenceMasking(t *testing.T) {
	ev := contracts.CrossInstitutionEvidence{
		PrimaryInstitutionID:   "INST-001",
		PrimaryInstitutionName: "Delhi Technological University",
		PrimaryProgramCode:     "BTECH-CSE",
		PrimaryAcademicYear:    "2024-25",
		OtherInstitutionID:     "INST-002",
		OtherInstitutionName:   "College of Engineering Pune",
		OtherProgramCode:       "BTECH-ECE",
		OtherAcademicYear:      "2024-25",
		MaskedAPAAR:            contracts.MaskAPAAR("987654321012"),
		MaskedDOB:              contracts.MaskDOB("2003-11-20"),
	}

	if ev.MaskedAPAAR != "9876****1012" {
		t.Errorf("expected 9876****1012, got %s", ev.MaskedAPAAR)
	}
	if ev.MaskedDOB != "2003-**-**" {
		t.Errorf("expected 2003-**-**, got %s", ev.MaskedDOB)
	}
}

func TestReviewDecisionEnum(t *testing.T) {
	decisions := []contracts.ReviewDecision{
		contracts.DecisionConfirmedSamePerson,
		contracts.DecisionConfirmedDifferentPeople,
		contracts.DecisionValidConcurrentStudy,
		contracts.DecisionNeedsMoreInformation,
		contracts.DecisionDismissed,
	}

	if len(decisions) != 5 {
		t.Errorf("expected 5 review decisions, got %d", len(decisions))
	}
}

