package upload

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"apaar-verify/internal/models"
	"apaar-verify/internal/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDuplicateIdempotency = errors.New("upload already submitted with this idempotency key")
	ErrMaxConcurrentJobs    = errors.New("concurrent upload limit reached: another batch is already processing for this institution")
)

const (
	BatchSize           = 500
	MaxConcurrentActive = 2
)

type StagingService struct {
	db *pgxpool.Pool
}

func NewStagingService(db *pgxpool.Pool) *StagingService {
	return &StagingService{db: db}
}

// StageUpload processes an incoming CSV stream with tenant isolation, bounds, and idempotency
func (s *StagingService) StageUpload(
	ctx context.Context,
	scope *tenant.Scope,
	targetInstitutionID string,
	filename string,
	idempotencyKey *string,
	fileReader io.Reader,
) (*models.UploadJob, []ParsedRowData, error) {
	// 1. Enforce tenant ownership (Non-negotiable Rule #2)
	if err := scope.EnforceInstitution(targetInstitutionID); err != nil {
		return nil, nil, err
	}

	// 2. Check concurrent active jobs for this tenant
	var activeCount int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM upload_jobs
		WHERE institution_id = $1 AND status IN ('queued', 'processing')
	`, targetInstitutionID).Scan(&activeCount)
	if err == nil && activeCount >= MaxConcurrentActive {
		return nil, nil, ErrMaxConcurrentJobs
	}

	// 3. Check idempotency if key provided (Non-negotiable Rule #10)
	if idempotencyKey != nil && *idempotencyKey != "" {
		var existingJob models.UploadJob
		checkQuery := `
			SELECT id, institution_id, filename, total_rows, processed_rows,
			       accepted_rows, flagged_rows, rejected_rows, status, created_at
			FROM upload_jobs
			WHERE institution_id = $1 AND idempotency_key = $2
		`
		err := s.db.QueryRow(ctx, checkQuery, targetInstitutionID, *idempotencyKey).Scan(
			&existingJob.ID,
			&existingJob.InstitutionID,
			&existingJob.Filename,
			&existingJob.TotalRows,
			&existingJob.ProcessedRows,
			&existingJob.AcceptedRows,
			&existingJob.FlaggedRows,
			&existingJob.RejectedRows,
			&existingJob.Status,
			&existingJob.CreatedAt,
		)
		if err == nil {
			// Idempotent hit: return existing job without creating any duplicates!
			existingJob.IdempotencyKey = idempotencyKey
			return &existingJob, nil, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, err
		}
	}

	// 4. Stream-parse CSV
	parsedRows, err := ParseCSVStream(fileReader)
	if err != nil {
		return nil, nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	// 5. Create upload_jobs record
	jobID := uuid.New()
	totalRows := len(parsedRows)
	var createdBy *uuid.UUID
	if scope.UserID != uuid.Nil {
		createdBy = &scope.UserID
	}

	insertJobQuery := `
		INSERT INTO upload_jobs (id, institution_id, created_by, filename, total_rows, status, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = tx.Exec(ctx, insertJobQuery, jobID, targetInstitutionID, createdBy, filename, totalRows, models.UploadStatusQueued, idempotencyKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create upload job: %w", err)
	}

	// 6. Persist EVERY row into upload_rows with raw submitted payload (Non-negotiable Rule #3)
	for _, row := range parsedRows {
		rawJSON, _ := json.Marshal(row.RawPayload)
		normJSON, _ := json.Marshal(row.NormalizedFields)
		var parseErrsJSON []byte
		if len(row.ParseErrors) > 0 {
			parseErrsJSON, _ = json.Marshal(row.ParseErrors)
		}

		insertRowQuery := `
			INSERT INTO upload_rows (id, job_id, institution_id, row_number, raw_payload, normalized_fields, parse_errors, outcome)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`
		_, err = tx.Exec(ctx, insertRowQuery, uuid.New(), jobID, targetInstitutionID, row.RowNumber, rawJSON, normJSON, parseErrsJSON, string(row.Outcome))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to persist upload row %d: %w", row.RowNumber, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	job := &models.UploadJob{
		ID:             jobID,
		InstitutionID:  targetInstitutionID,
		CreatedBy:      createdBy,
		Filename:       filename,
		TotalRows:      totalRows,
		Status:         models.UploadStatusQueued,
		IdempotencyKey: idempotencyKey,
	}

	return job, parsedRows, nil
}
