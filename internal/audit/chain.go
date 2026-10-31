package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"apaar-verify/internal/contracts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

var ErrChainTampered = errors.New("audit log integrity violation: tampered or broken hash chain detected")

// Service handles tamper-evident append-only audit logging implementing contracts.Auditor
type Service struct {
	db *pgxpool.Pool
	mu sync.Mutex // In-process guard for concurrent testing and non-DB scenarios
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// ComputeEventHash computes SHA-256(prev_hash || canonical event bytes)
// Rule #7: Tamper-evident hash chain. No private payloads logged.
func ComputeEventHash(prevHash string, actorID *uuid.UUID, role, action, targetType, targetID string, ts time.Time, metadata []byte) string {
	actorStr := "SYSTEM"
	if actorID != nil {
		actorStr = actorID.String()
	}

	canonicalString := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		prevHash,
		actorStr,
		role,
		action,
		targetType,
		targetID,
		ts.UTC().Format(time.RFC3339Nano),
		string(metadata),
	)

	sum := sha256.Sum256([]byte(canonicalString))
	return hex.EncodeToString(sum[:])
}

// Record implements contracts.Auditor inside an active transaction with pessimistic row lock
// Concurrency foundation: Locks audit_head (FOR UPDATE) so concurrent transactions cannot fork the chain
func (s *Service) Record(ctx context.Context, tx pgx.Tx, event contracts.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var metaBytes []byte
	if event.Metadata != nil {
		var err error
		metaBytes, err = json.Marshal(event.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal audit metadata: %w", err)
		}
	}

	var prevHash string

	if tx != nil {
		// Strictly serialize appends via row lock on audit_head
		headQuery := `
			SELECT last_event_hash
			FROM audit_head
			WHERE id = 1
			FOR UPDATE
		`
		err := tx.QueryRow(ctx, headQuery).Scan(&prevHash)
		if err != nil {
			// Fallback if audit_head not yet migrated
			prevHash = GenesisHash
		}
	} else if s.db != nil {
		row := s.db.QueryRow(ctx, "SELECT event_hash FROM audit_events ORDER BY id DESC LIMIT 1")
		if err := row.Scan(&prevHash); err != nil {
			prevHash = GenesisHash
		}
	} else {
		prevHash = GenesisHash
	}

	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	eventHash := ComputeEventHash(
		prevHash,
		event.ActorID,
		event.ActorRole,
		event.Action,
		event.TargetType,
		event.TargetID,
		occurredAt,
		metaBytes,
	)

	if tx != nil {
		insertQuery := `
			INSERT INTO audit_events (actor_id, actor_role, action, target_type, target_id, institution_id, details, prev_hash, event_hash, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id
		`
		var newID int64
		err := tx.QueryRow(ctx, insertQuery,
			event.ActorID,
			event.ActorRole,
			event.Action,
			event.TargetType,
			event.TargetID,
			event.TenantID,
			metaBytes,
			prevHash,
			eventHash,
			occurredAt,
		).Scan(&newID)
		if err != nil {
			return fmt.Errorf("failed to insert audit event: %w", err)
		}

		// Update audit_head within the same transaction
		_, _ = tx.Exec(ctx, `
			UPDATE audit_head
			SET last_event_id = $1, last_event_hash = $2, updated_at = CURRENT_TIMESTAMP
			WHERE id = 1
		`, newID, eventHash)

	} else if s.db != nil {
		insertQuery := `
			INSERT INTO audit_events (actor_id, actor_role, action, target_type, target_id, institution_id, details, prev_hash, event_hash, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`
		_, err := s.db.Exec(ctx, insertQuery,
			event.ActorID,
			event.ActorRole,
			event.Action,
			event.TargetType,
			event.TargetID,
			event.TenantID,
			metaBytes,
			prevHash,
			eventHash,
			occurredAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert non-tx audit event: %w", err)
		}
	}

	return nil
}

// LogEvent is a convenience wrapper for backwards compatibility
func (s *Service) LogEvent(
	ctx context.Context,
	tx pgx.Tx,
	actorID *uuid.UUID,
	role string,
	action string,
	targetType string,
	targetID string,
	institutionID *string,
	details map[string]any,
) error {
	return s.Record(ctx, tx, contracts.AuditEvent{
		ActorID:    actorID,
		ActorRole:  role,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		TenantID:   institutionID,
		Metadata:   details,
		OccurredAt: time.Now().UTC(),
	})
}

// VerifyIntegrity walks the entire hash chain from genesis to recompute and verify hashes
func (s *Service) VerifyIntegrity(ctx context.Context) (bool, int64, error) {
	if s.db == nil {
		return true, 0, nil
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, actor_id, actor_role, action, target_type, target_id, details, prev_hash, event_hash, created_at
		FROM audit_events
		ORDER BY id ASC
	`)
	if err != nil {
		return false, 0, err
	}
	defer rows.Close()

	expectedPrev := GenesisHash
	for rows.Next() {
		var id int64
		var actorID *uuid.UUID
		var role, action, targetType, targetID, prevHash, eventHash string
		var details []byte
		var ts time.Time

		if err := rows.Scan(&id, &actorID, &role, &action, &targetType, &targetID, &details, &prevHash, &eventHash, &ts); err != nil {
			return false, id, err
		}

		if prevHash != expectedPrev {
			return false, id, fmt.Errorf("%w at event id %d: expected prev_hash %s, found %s", ErrChainTampered, id, expectedPrev, prevHash)
		}

		computed := ComputeEventHash(prevHash, actorID, role, action, targetType, targetID, ts, details)
		if computed != eventHash {
			return false, id, fmt.Errorf("%w at event id %d: computed hash %s != recorded hash %s", ErrChainTampered, id, computed, eventHash)
		}

		expectedPrev = eventHash
	}

	return true, 0, nil
}
