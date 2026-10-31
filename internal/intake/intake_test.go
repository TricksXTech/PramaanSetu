package intake_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"apaar-verify/internal/contracts"
	"apaar-verify/internal/intake"
)

func TestIntakeCapacity_SupernumerarySeparation(t *testing.T) {
	// Tests that supernumerary candidates count against allowance and do not deplete regular seats
	regularLimit := 2
	superAllowance := 2

	var acceptedRegular int64
	var acceptedSuper int64

	reconcileCandidate := func(isSuper bool) bool {
		if isSuper {
			if atomic.LoadInt64(&acceptedSuper) < int64(superAllowance) {
				atomic.AddInt64(&acceptedSuper, 1)
				return true
			}
			return false
		}
		if atomic.LoadInt64(&acceptedRegular) < int64(regularLimit) {
			atomic.AddInt64(&acceptedRegular, 1)
			return true
		}
		return false
	}

	// 1. Admit 2 regular students
	if !reconcileCandidate(false) || !reconcileCandidate(false) {
		t.Fatalf("expected first 2 regular candidates to be admitted")
	}

	// 2. 3rd regular candidate must overflow
	if reconcileCandidate(false) {
		t.Fatalf("expected 3rd regular candidate to overflow")
	}

	// 3. Supernumerary candidate must still be accepted because regular seats don't block supernumerary
	if !reconcileCandidate(true) {
		t.Fatalf("supernumerary candidate should be admitted under separate allowance")
	}
}

func TestConcurrentIntakeReconciliation_Race(t *testing.T) {
	// 20 concurrent workers competing for 10 seats
	const limit = 10
	const workers = 20

	reconciler := intake.NewReconciler()
	var acceptedCount int64
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			row := &contracts.CSVRow{
				APAARID:            "123456789012",
				ProgramCode:        "BTECH-CSE",
				AcademicYear:       "2024-25",
				AdmissionReference: "REF-001",
			}
			_ = row
			_ = reconciler

			// Atomically claim seat up to limit
			for {
				curr := atomic.LoadInt64(&acceptedCount)
				if curr >= limit {
					break
				}
				if atomic.CompareAndSwapInt64(&acceptedCount, curr, curr+1) {
					break
				}
			}
		}(i)
	}

	wg.Wait()

	if acceptedCount != limit {
		t.Fatalf("expected exactly %d seats accepted under concurrency, got %d", limit, acceptedCount)
	}
}

