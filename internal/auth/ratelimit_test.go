package auth_test

import (
	"testing"
	"time"

	"apaar-verify/internal/auth"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := auth.NewRateLimiter(5, 100*time.Millisecond)

	// First 5 requests must succeed
	for i := 0; i < 5; i++ {
		allowed, _ := rl.Allow("client-1")
		if !allowed {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}

	// 6th request must be denied with Retry-After > 0
	allowed, retryAfter := rl.Allow("client-1")
	if allowed {
		t.Fatalf("6th request exceeded limit, expected rejection")
	}
	if retryAfter <= 0 {
		t.Errorf("expected retryAfter > 0, got %d", retryAfter)
	}

	// Different client must still be allowed
	allowed2, _ := rl.Allow("client-2")
	if !allowed2 {
		t.Fatalf("different client should not be affected by client-1 limit")
	}

	// After window expires, client-1 must be allowed again
	time.Sleep(120 * time.Millisecond)
	allowedAfterWait, _ := rl.Allow("client-1")
	if !allowedAfterWait {
		t.Fatalf("request after window expiry should have been allowed")
	}
}

