package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"apaar-verify/internal/contracts"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidSignature = errors.New("invalid webhook signature")
	ErrTimestampExpired = errors.New("webhook timestamp expired or outside tolerance window")
	ErrDuplicateEvent   = errors.New("duplicate webhook event ignored")
)

// Dispatcher handles HMAC-SHA256 signed flag webhook delivery with bounded retries
type Dispatcher struct {
	db          *pgxpool.Pool
	secret      []byte
	receiverURL string
	client      *http.Client
}

func NewDispatcher(db *pgxpool.Pool, secret string, receiverURL string) *Dispatcher {
	if secret == "" {
		secret = os.Getenv("WEBHOOK_SECRET")
		if secret == "" {
			secret = "whsec_demo_secret_key_12345"
		}
	}
	if receiverURL == "" {
		receiverURL = os.Getenv("WEBHOOK_RECEIVER_URL")
		if receiverURL == "" {
			receiverURL = "http://localhost:8080/demo/webhook-receiver"
		}
	}
	return &Dispatcher{
		db:          db,
		secret:      []byte(secret),
		receiverURL: receiverURL,
		client:      &http.Client{Timeout: 5 * time.Second},
	}
}

// ComputeSignature calculates HMAC-SHA256 signature for the given body
func ComputeSignature(secret []byte, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature verifies the HMAC-SHA256 signature against the raw body
func VerifySignature(secret []byte, body []byte, signatureHeader string) bool {
	expected := ComputeSignature(secret, body)
	return hmac.Equal([]byte(expected), []byte(signatureHeader))
}

// DispatchFlagEvent serializes, signs, and delivers a flag creation webhook
// Strict Non-negotiable Rule #7: Body contains identifiers only, ZERO names, DOB, or APAAR IDs
func (d *Dispatcher) DispatchFlagEvent(ctx context.Context, event contracts.WebhookEvent, forceFailure bool) error {
	payloadBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	sig := ComputeSignature(d.secret, payloadBytes)
	timestampStr := event.OccurredAt.UTC().Format(time.RFC3339)

	// Attempt delivery with bounded exponential backoff (max 3 attempts)
	maxAttempts := 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.receiverURL, bytes.NewReader(payloadBytes))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Event-Id", event.EventID.String())
		req.Header.Set("X-Event-Timestamp", timestampStr)
		req.Header.Set("X-Signature", sig)
		if forceFailure {
			req.Header.Set("X-Force-Failure", "true")
		}

		resp, err := d.client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				// Success: persist delivery record if db is available
				d.recordDelivery(ctx, event, payloadBytes, sig, attempt, "delivered", nil)
				return nil
			}
			lastErr = fmt.Errorf("receiver returned HTTP %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		// Exponential backoff
		if attempt < maxAttempts {
			backoffMs := int(math.Pow(2, float64(attempt))) * 20
			time.Sleep(time.Duration(backoffMs) * time.Millisecond)
		}
	}

	// Delivery failed after max attempts
	errStr := lastErr.Error()
	d.recordDelivery(ctx, event, payloadBytes, sig, maxAttempts, "failed", &errStr)
	return fmt.Errorf("webhook delivery failed after %d attempts: %w", maxAttempts, lastErr)
}

func (d *Dispatcher) recordDelivery(
	ctx context.Context,
	event contracts.WebhookEvent,
	payload []byte,
	signature string,
	attempts int,
	status string,
	lastErr *string,
) {
	if d.db == nil {
		return
	}
	query := `
		INSERT INTO webhook_deliveries (
			event_id, event_type, institution_id, flag_id, rule_code, severity, payload, signature, attempts, status, last_error, delivered_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, CURRENT_TIMESTAMP)
		ON CONFLICT (event_id) DO UPDATE SET
			attempts = EXCLUDED.attempts,
			status = EXCLUDED.status,
			last_error = EXCLUDED.last_error,
			delivered_at = CURRENT_TIMESTAMP
	`
	_, _ = d.db.Exec(ctx, query,
		event.EventID,
		event.EventType,
		event.InstitutionID,
		event.FlagID,
		event.RuleCode,
		event.Severity,
		payload,
		signature,
		attempts,
		status,
		lastErr,
	)
}

// Receiver manages verified incoming webhook events and deduplication
type Receiver struct {
	mu            sync.Mutex
	secret        []byte
	tolerance     time.Duration
	seenEvents    map[string]time.Time
	recentEvents  []contracts.WebhookEvent
	maxStoreCount int
}

func NewReceiver(secret string, tolerance time.Duration) *Receiver {
	if secret == "" {
		secret = "whsec_demo_secret_key_12345"
	}
	if tolerance <= 0 {
		tolerance = 5 * time.Minute
	}
	return &Receiver{
		secret:        []byte(secret),
		tolerance:     tolerance,
		seenEvents:    make(map[string]time.Time),
		recentEvents:  make([]contracts.WebhookEvent, 0),
		maxStoreCount: 50,
	}
}

// ProcessRequest validates the incoming webhook, checks signature, timestamp, and deduplication
func (r *Receiver) ProcessRequest(req *http.Request) (*contracts.WebhookEvent, error) {
	if req.Header.Get("X-Force-Failure") == "true" {
		return nil, errors.New("forced failure for demo retry inspection")
	}

	eventIDStr := req.Header.Get("X-Event-Id")
	timestampStr := req.Header.Get("X-Event-Timestamp")
	signature := req.Header.Get("X-Signature")

	if eventIDStr == "" || timestampStr == "" || signature == "" {
		return nil, errors.New("missing required webhook headers (X-Event-Id, X-Event-Timestamp, X-Signature)")
	}

	eventTime, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return nil, errors.New("invalid timestamp format")
	}

	// 1. Check timestamp tolerance window
	now := time.Now().UTC()
	if now.Sub(eventTime) > r.tolerance || eventTime.Sub(now) > r.tolerance {
		return nil, ErrTimestampExpired
	}

	// 2. Read and verify signature
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	if !VerifySignature(r.secret, body, signature) {
		return nil, ErrInvalidSignature
	}

	// 3. Deduplication by Event ID
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.seenEvents[eventIDStr]; exists {
		return nil, ErrDuplicateEvent
	}

	var event contracts.WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}

	r.seenEvents[eventIDStr] = now
	r.recentEvents = append([]contracts.WebhookEvent{event}, r.recentEvents...)
	if len(r.recentEvents) > r.maxStoreCount {
		r.recentEvents = r.recentEvents[:r.maxStoreCount]
	}

	return &event, nil
}

// RecentEvents returns the list of received events for demo inspection
func (r *Receiver) RecentEvents() []contracts.WebhookEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := make([]contracts.WebhookEvent, len(r.recentEvents))
	copy(copied, r.recentEvents)
	return copied
}
