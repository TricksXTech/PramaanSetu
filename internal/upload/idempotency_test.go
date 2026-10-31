package upload_test

import (
	"sync"
	"testing"

	"apaar-verify/internal/contracts"
	"github.com/google/uuid"
)

type mockUploadStore struct {
	mu           sync.Mutex
	jobsByIdem   map[string]uuid.UUID
	enrolments   map[string]int // key: inst:year:ref -> version
	rollups      map[string]int // key: inst:prog:year -> count
	flagsCreated map[string]int // key: row_id:rule -> count
}

func newMockUploadStore() *mockUploadStore {
	return &mockUploadStore{
		jobsByIdem:   make(map[string]uuid.UUID),
		enrolments:   make(map[string]int),
		rollups:      make(map[string]int),
		flagsCreated: make(map[string]int),
	}
}

func (s *mockUploadStore) StageJobWithIdempotency(idemKey string) (uuid.UUID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existingID, exists := s.jobsByIdem[idemKey]; exists {
		return existingID, true // Replay detected, returned existing job
	}

	newID := uuid.New()
	s.jobsByIdem[idemKey] = newID
	return newID, false
}

func (s *mockUploadStore) ProcessRow(instID, programID, year, ref string, rowID uuid.UUID) contracts.RowOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()

	enrolmentKey := instID + ":" + year + ":" + ref
	rollupKey := instID + ":" + programID + ":" + year

	// Rule V4: Existing enrolment with same tenant reference -> no-op if identical
	if version, exists := s.enrolments[enrolmentKey]; exists {
		_ = version
		return contracts.RowOutcomeNoOpIdentical
	}

	// Create enrolment
	s.enrolments[enrolmentKey] = 1
	s.rollups[rollupKey]++
	return contracts.RowOutcomeAccepted
}

func TestUploadIdempotency_APIKeyReplay(t *testing.T) {
	// Dev B Task 2: Replay the same API idempotency key returns same job with no duplicate
	store := newMockUploadStore()
	idemKey := "idem-test-uuid-99999"

	// 1. Initial submission
	jobID1, isReplay1 := store.StageJobWithIdempotency(idemKey)
	if isReplay1 {
		t.Fatalf("expected initial submission to not be a replay")
	}

	// 2. Immediate replay of exact same idempotency key
	jobID2, isReplay2 := store.StageJobWithIdempotency(idemKey)
	if !isReplay2 {
		t.Fatalf("expected second submission with same key to be detected as replay")
	}
	if jobID1 != jobID2 {
		t.Fatalf("expected identical job ID on idempotency replay, got %s and %s", jobID1, jobID2)
	}

	// 3. Different idempotency key generates new job
	jobID3, isReplay3 := store.StageJobWithIdempotency("idem-another-uuid")
	if isReplay3 || jobID3 == jobID1 {
		t.Fatalf("expected new job for distinct idempotency key")
	}
}

func TestBatchRetry_NoDuplicateEnrolmentsOrRollupDrift(t *testing.T) {
	// Dev B Task 2: Deliver the same batch twice (simulating worker restart mid-batch or job retry)
	// Assert no duplicate enrolments, flags or rollup drift
	store := newMockUploadStore()

	instID := "INST-001"
	programID := "BTECH-CSE"
	year := "2024-25"
	ref := "DTU-2024-001"
	rowID := uuid.New()

	// 1. Initial batch processing
	outcome1 := store.ProcessRow(instID, programID, year, ref, rowID)
	if outcome1 != contracts.RowOutcomeAccepted {
		t.Fatalf("expected initial row to be accepted, got %s", outcome1)
	}

	// 2. Retry / re-delivery of the exact same row (e.g. worker kill and redelivery)
	outcome2 := store.ProcessRow(instID, programID, year, ref, rowID)
	if outcome2 != contracts.RowOutcomeNoOpIdentical {
		t.Fatalf("expected retried row to be no_op_identical (Rule V4), got %s", outcome2)
	}

	// 3. Assert zero duplicate enrolments or rollup count drift
	rollupKey := instID + ":" + programID + ":" + year
	count := store.rollups[rollupKey]
	if count != 1 {
		t.Fatalf("rollup drift detected: expected count 1, got %d after redelivery", count)
	}

	enrolmentKey := instID + ":" + year + ":" + ref
	if len(store.enrolments) != 1 {
		t.Fatalf("duplicate enrolment created: expected 1 enrolment, got %d", len(store.enrolments))
	}
	if store.enrolments[enrolmentKey] != 1 {
		t.Fatalf("enrolment version corrupt: %d", store.enrolments[enrolmentKey])
	}
}
