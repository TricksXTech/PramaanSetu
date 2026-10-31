package auth

import (
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var ErrRateLimitExceeded = errors.New("rate limit exceeded")

type entry struct {
	timestamps []time.Time
}

// RateLimiter provides thread-safe in-memory sliding window rate limiting
type RateLimiter struct {
	mu      sync.Mutex
	limit   int           // Max requests per window
	window  time.Duration // Time window
	clients map[string]*entry
}

// NewRateLimiter creates a RateLimiter with specified request limit per window
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{
		limit:   limit,
		window:  window,
		clients: make(map[string]*entry),
	}
}

// Allow evaluates if a request from key is permitted under the rate limit
func (rl *RateLimiter) Allow(key string) (bool, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	e, exists := rl.clients[key]
	if !exists {
		rl.clients[key] = &entry{timestamps: []time.Time{now}}
		return true, 0
	}

	// Filter timestamps outside window
	valid := make([]time.Time, 0, len(e.timestamps))
	for _, t := range e.timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		e.timestamps = valid
		// Calculate Retry-After in seconds
		oldest := valid[0]
		retryAfter := int(oldest.Add(rl.window).Sub(now).Seconds()) + 1
		if retryAfter <= 0 {
			retryAfter = 1
		}
		return false, retryAfter
	}

	valid = append(valid, now)
	e.timestamps = valid
	return true, 0
}

// RateLimitMiddleware enforces rate limiting per client IP or API key
func RateLimitMiddleware(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			key = c.ClientIP()
		} else {
			key = HashToken(key)
		}

		allowed, retryAfter := rl.Allow(key)
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Too Many Requests",
				"message":     "API rate limit exceeded. Please back off.",
				"retry_after": retryAfter,
			})
			return
		}
		c.Next()
	}
}
