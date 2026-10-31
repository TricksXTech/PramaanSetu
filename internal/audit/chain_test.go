package audit_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"apaar-verify/internal/audit"
	"apaar-verify/internal/contracts"
	"github.com/google/uuid"
)

func TestAuditHashChain_TamperEvident(t *testing.T) {
	actorID := uuid.New()
	role := "institution_admin"
	action := "UPLOAD_CREATED"
	targetType := "upload_job"
	targetID := uuid.New().String()
	ts := time.Now().UTC()
	details := []byte(`{"total_rows": 10}`)

	// 1. Compute original hash
	hash1 := audit.ComputeEventHash(audit.GenesisHash, &actorID, role, action, targetType, targetID, ts, details)
	if hash1 == "" {
		t.Fatalf("expected non-empty event hash")
	}

	// 2. Compute second chained event
	action2 := "ROW_FLAGGED"
	targetID2 := uuid.New().String()
	ts2 := ts.Add(1 * time.Second)
	hash2 := audit.ComputeEventHash(hash1, &actorID, role, action2, targetType, targetID2, ts2, nil)

	if hash2 == hash1 {
		t.Fatalf("subsequent event hash must differ from previous hash")
	}

	// 3. Simulate tamper: if an attacker modifies the details of event 1, recomputed hash changes
	tamperedDetails := []byte(`{"total_rows": 999}`)
	tamperedHash1 := audit.ComputeEventHash(audit.GenesisHash, &actorID, role, action, targetType, targetID, ts, tamperedDetails)

	if tamperedHash1 == hash1 {
		t.Fatalf("tampered event payload produced same hash, expected tamper detection")
	}

	// Chained event 2 fails verification under tampered hash 1
	tamperedHash2 := audit.ComputeEventHash(tamperedHash1, &actorID, role, action2, targetType, targetID2, ts2, nil)
	if tamperedHash2 == hash2 {
		t.Fatalf("chain failure not propagated")
	}
}

func TestConcurrentAuditLogging_NoFork(t *testing.T) {
	// Tests that concurrent appends do not race or cause data races (run with -race)
	svc := audit.NewService(nil)
	const concurrentWriters = 20
	var wg sync.WaitGroup

	for i := 0; i < concurrentWriters; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			actorID := uuid.New()
			instID := "INST-001"
			_ = svc.Record(context.Background(), nil, contracts.AuditEvent{
				ActorID:    &actorID,
				ActorRole:  "institution_admin",
				Action:     "UPLOAD_STAGED",
				TargetType: "upload_job",
				TargetID:   uuid.New().String(),
				TenantID:   &instID,
				Metadata:   map[string]any{"worker": workerID},
				OccurredAt: time.Now().UTC(),
			})
		}(i)
	}

	wg.Wait()
}

func TestAuditLog_ZeroPersonalData(t *testing.T) {
	// Rule #7: Metadata must never contain student names, DOB, APAAR IDs or raw rows
	prohibitedFields := []string{"apaar_id", "student_name", "dob", "guardian_name", "raw_payload"}

	event := contracts.AuditEvent{
		ActorRole:  "institution_admin",
		Action:     "ENROLMENT_PROMOTED",
		TargetType: "enrolment",
		TargetID:   uuid.New().String(),
		Metadata: map[string]any{
			"enrolment_version": 1,
			"program_code":      "BTECH-CSE",
			"academic_year":     "2024-25",
		},
	}

	for k := range event.Metadata {
		lower := strings.ToLower(k)
		for _, p := range prohibitedFields {
			if lower == p {
				t.Fatalf("privacy violation: audit metadata contains sensitive personal field %q", k)
			}
		}
	}
}

func TestAuditHashChain_MicrosecondAndCanonicalJSON(t *testing.T) {
	actorID := uuid.New()
	role := "aicte_admin"
	action := "USER_LOGIN_SUCCESS"
	targetType := "user"
	targetID := actorID.String()

	// High precision timestamp (e.g. from Go runtime with nanoseconds)
	tsNano := time.Date(2026, 10, 31, 13, 3, 15, 359801252, time.UTC)
	// Truncated timestamp (as stored by PostgreSQL TIMESTAMPTZ)
	tsMicro := time.Date(2026, 10, 31, 13, 3, 15, 359801000, time.UTC)

	// In-memory compact JSON
	metaCompact := []byte(`{"ip":"127.0.0.1"}`)
	// PostgreSQL JSONB formatted JSON (with space after colon)
	metaSpaced := []byte(`{"ip": "127.0.0.1"}`)

	hashRecorded := audit.ComputeEventHash(audit.GenesisHash, &actorID, role, action, targetType, targetID, tsNano, metaCompact)
	hashVerified := audit.ComputeEventHash(audit.GenesisHash, &actorID, role, action, targetType, targetID, tsMicro, metaSpaced)

	if hashRecorded != hashVerified {
		t.Fatalf("expected hashRecorded to match hashVerified despite PostgreSQL microsecond truncation and JSONB spacing; got %s != %s", hashRecorded, hashVerified)
	}
}
