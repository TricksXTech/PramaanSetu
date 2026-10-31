package webhook_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"apaar-verify/internal/contracts"
	"apaar-verify/internal/webhook"
	"github.com/google/uuid"
)

func TestWebhook_HMACSigningAndVerification(t *testing.T) {
	secret := "test_secret_12345"
	receiver := webhook.NewReceiver(secret, 5*time.Minute)

	event := contracts.WebhookEvent{
		EventID:       uuid.New(),
		EventType:     "flag.created",
		OccurredAt:    time.Now().UTC(),
		InstitutionID: "INST-001",
		FlagID:        uuid.New(),
		RuleCode:      "V7",
		Severity:      "warning",
	}

	payload, _ := json.Marshal(event)
	sig := webhook.ComputeSignature([]byte(secret), payload)
	timestampStr := event.OccurredAt.Format(time.RFC3339)

	// 1. Valid request -> Succeeded
	req1 := httptest.NewRequest(http.MethodPost, "/demo/webhook-receiver", bytes.NewReader(payload))
	req1.Header.Set("X-Event-Id", event.EventID.String())
	req1.Header.Set("X-Event-Timestamp", timestampStr)
	req1.Header.Set("X-Signature", sig)

	received, err := receiver.ProcessRequest(req1)
	if err != nil {
		t.Fatalf("expected successful processing, got: %v", err)
	}
	if received.EventID != event.EventID {
		t.Fatalf("expected event ID %s, got %s", event.EventID, received.EventID)
	}

	// 2. Tampered body -> Must fail signature verification
	tamperedPayload := make([]byte, len(payload))
	copy(tamperedPayload, payload)
	tamperedPayload[len(tamperedPayload)-1] ^= 0xFF

	req2 := httptest.NewRequest(http.MethodPost, "/demo/webhook-receiver", bytes.NewReader(tamperedPayload))
	req2.Header.Set("X-Event-Id", uuid.New().String())
	req2.Header.Set("X-Event-Timestamp", timestampStr)
	req2.Header.Set("X-Signature", sig)

	_, err = receiver.ProcessRequest(req2)
	if err != webhook.ErrInvalidSignature {
		t.Fatalf("expected ErrInvalidSignature for tampered payload, got: %v", err)
	}

	// 3. Replay of same event ID -> Must be detected as duplicate
	req3 := httptest.NewRequest(http.MethodPost, "/demo/webhook-receiver", bytes.NewReader(payload))
	req3.Header.Set("X-Event-Id", event.EventID.String())
	req3.Header.Set("X-Event-Timestamp", timestampStr)
	req3.Header.Set("X-Signature", sig)

	_, err = receiver.ProcessRequest(req3)
	if err != webhook.ErrDuplicateEvent {
		t.Fatalf("expected ErrDuplicateEvent for duplicate event ID, got: %v", err)
	}

	// 4. Stale timestamp (outside 5 min tolerance window) -> Must be rejected
	staleTime := time.Now().UTC().Add(-10 * time.Minute)
	eventStale := contracts.WebhookEvent{
		EventID:       uuid.New(),
		EventType:     "flag.created",
		OccurredAt:    staleTime,
		InstitutionID: "INST-001",
		FlagID:        uuid.New(),
		RuleCode:      "V8",
		Severity:      "warning",
	}
	stalePayload, _ := json.Marshal(eventStale)
	staleSig := webhook.ComputeSignature([]byte(secret), stalePayload)

	req4 := httptest.NewRequest(http.MethodPost, "/demo/webhook-receiver", bytes.NewReader(stalePayload))
	req4.Header.Set("X-Event-Id", eventStale.EventID.String())
	req4.Header.Set("X-Event-Timestamp", staleTime.Format(time.RFC3339))
	req4.Header.Set("X-Signature", staleSig)

	_, err = receiver.ProcessRequest(req4)
	if err != webhook.ErrTimestampExpired {
		t.Fatalf("expected ErrTimestampExpired for stale timestamp, got: %v", err)
	}

	// 5. Force failure header -> Returns error
	req5 := httptest.NewRequest(http.MethodPost, "/demo/webhook-receiver", bytes.NewReader(payload))
	req5.Header.Set("X-Event-Id", uuid.New().String())
	req5.Header.Set("X-Event-Timestamp", time.Now().UTC().Format(time.RFC3339))
	req5.Header.Set("X-Signature", sig)
	req5.Header.Set("X-Force-Failure", "true")

	_, err = receiver.ProcessRequest(req5)
	if err == nil {
		t.Fatalf("expected error on forced failure, but got nil")
	}
}

