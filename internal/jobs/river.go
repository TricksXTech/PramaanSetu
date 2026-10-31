package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"apaar-verify/internal/contracts"
	"apaar-verify/internal/validate"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// UploadBatchArgs defines the payload for processing a chunk of rows
type UploadBatchArgs struct {
	JobID         uuid.UUID `json:"job_id"`
	InstitutionID string    `json:"institution_id"`
	StartRow      int       `json:"start_row"`
	EndRow        int       `json:"end_row"`
}

func (UploadBatchArgs) Kind() string { return "upload_batch" }

func (UploadBatchArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: 3,
		Priority:    1,
	}
}

// UploadBatchWorker processes staged rows through validation and promotion
type UploadBatchWorker struct {
	river.WorkerDefaults[UploadBatchArgs]
	db        *pgxpool.Pool
	validator contracts.Validator
	policyCfg contracts.PolicyConfig
}

func NewUploadBatchWorker(db *pgxpool.Pool, validator contracts.Validator) *UploadBatchWorker {
	if validator == nil {
		validator = validate.NewEngine(nil)
	}
	return &UploadBatchWorker{
		db:        db,
		validator: validator,
		policyCfg: validate.LoadPolicyConfig(""),
	}
}

// Work implements river.Worker with strict row state idempotency (Rule #10)
func (w *UploadBatchWorker) Work(ctx context.Context, job *river.Job[UploadBatchArgs]) error {
	return ProcessBatch(ctx, w.db, w.validator, &w.policyCfg, job.Args.JobID, job.Args.InstitutionID, job.Args.StartRow, job.Args.EndRow)
}

// ProcessBatch processes a range of staged rows inside a database transaction
func ProcessBatch(
	ctx context.Context,
	db *pgxpool.Pool,
	validator contracts.Validator,
	policyCfg *contracts.PolicyConfig,
	jobID uuid.UUID,
	institutionID string,
	startRow, endRow int,
) error {
	log.Printf("[Worker] Processing batch for job %s rows %d-%d", jobID, startRow, endRow)

	if policyCfg == nil {
		cfg := validate.LoadPolicyConfig("")
		policyCfg = &cfg
	}

	// Update job state to processing
	_, _ = db.Exec(ctx, `
		UPDATE upload_jobs SET status = 'processing', updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND status = 'queued'
	`, jobID)

	// Fetch only staged rows in this batch window (Idempotent worker execution)
	query := `
		SELECT id, row_number, normalized_fields
		FROM upload_rows
		WHERE job_id = $1 AND row_number BETWEEN $2 AND $3 AND outcome = 'staged'
		ORDER BY row_number ASC
	`
	rows, err := db.Query(ctx, query, jobID, startRow, endRow)
	if err != nil {
		return fmt.Errorf("failed to fetch staged rows: %w", err)
	}
	defer rows.Close()

	type rowItem struct {
		id        uuid.UUID
		rowNumber int
		fields    contracts.CSVRow
	}
	var batchRows []rowItem

	for rows.Next() {
		var item rowItem
		var rawNorm []byte
		if err := rows.Scan(&item.id, &item.rowNumber, &rawNorm); err == nil {
			_ = json.Unmarshal(rawNorm, &item.fields)
			batchRows = append(batchRows, item)
		}
	}
	rows.Close()

	if len(batchRows) == 0 {
		checkJobCompletion(ctx, db, jobID)
		return nil
	}

	seenMap := make(map[string]int)

	// Process each row in its own transactional unit
	for _, item := range batchRows {
		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}

		meta := &contracts.ValidationMetadata{
			JobID:         jobID,
			InstitutionID: institutionID,
			UploadRowID:   item.id,
			RowNumber:     item.rowNumber,
			SeenInFile:    seenMap,
			PolicyConfig:  policyCfg,
		}

		flags, outcome, valErr := validator.Validate(ctx, tx, &item.fields, meta)
		if valErr != nil {
			tx.Rollback(ctx)
			continue
		}

		var enrolmentID *uuid.UUID

		if outcome == contracts.RowOutcomeAccepted {
			eid, promoFlags, err := validate.PromoteRow(ctx, tx, institutionID, item.id, &item.fields)
			if err != nil {
				tx.Rollback(ctx)
				continue
			}
			enrolmentID = &eid
			flags = append(flags, promoFlags...)
		} else if outcome == contracts.RowOutcomeFlaggedHeld {
			// Update rollup held count inside transaction
			_ = validate.RecordRollupHeld(ctx, tx, institutionID, item.fields.ProgramCode, item.fields.AcademicYear)
		}

		// Persist validation flags if any
		for _, f := range flags {
			evidenceJSON, _ := json.Marshal(f.Evidence)
			polVer := f.PolicyVersion
			if polVer == "" {
				polVer = policyCfg.PolicyVersion
			}
			insertFlag := `
				INSERT INTO validation_flags (upload_row_id, institution_id, rule_code, severity, reason, evidence, policy_version)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`
			_, _ = tx.Exec(ctx, insertFlag, item.id, institutionID, f.RuleCode, f.Severity, f.Reason, evidenceJSON, polVer)
		}

		// Update row outcome (Non-negotiable Rule #3)
		updateRow := `
			UPDATE upload_rows
			SET outcome = $1, enrolment_id = $2, updated_at = CURRENT_TIMESTAMP
			WHERE id = $3
		`
		_, _ = tx.Exec(ctx, updateRow, string(outcome), enrolmentID, item.id)

		// Increment counts on upload_jobs
		var isAccepted, isFlagged, isRejected int
		if outcome == contracts.RowOutcomeAccepted || outcome == contracts.RowOutcomeNoOpIdentical {
			isAccepted = 1
		} else if outcome == contracts.RowOutcomeFlaggedHeld || outcome == contracts.RowOutcomeConflictStale || outcome == contracts.RowOutcomePendingRegistry {
			isFlagged = 1
		} else if outcome == contracts.RowOutcomeRejectedInvalid || outcome == contracts.RowOutcomeRejectedParse {
			isRejected = 1
		}

		updateJobCounts := `
			UPDATE upload_jobs
			SET processed_rows = processed_rows + 1,
			    accepted_rows = accepted_rows + $1,
			    flagged_rows = flagged_rows + $2,
			    rejected_rows = rejected_rows + $3,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = $4
		`
		_, _ = tx.Exec(ctx, updateJobCounts, isAccepted, isFlagged, isRejected, jobID)

		_ = tx.Commit(ctx)
	}

	checkJobCompletion(ctx, db, jobID)
	return nil
}

func checkJobCompletion(ctx context.Context, db *pgxpool.Pool, jobID uuid.UUID) {
	var total, processed int
	err := db.QueryRow(ctx, "SELECT total_rows, processed_rows FROM upload_jobs WHERE id = $1", jobID).Scan(&total, &processed)
	if err == nil && processed >= total {
		_, _ = db.Exec(ctx, "UPDATE upload_jobs SET status = 'completed', updated_at = CURRENT_TIMESTAMP WHERE id = $1", jobID)
	}
}

// SetupRiver initializes River migrations and client runner
func SetupRiver(ctx context.Context, dbPool *pgxpool.Pool, validator contracts.Validator) (*river.Client[pgx.Tx], error) {
	driver := riverpgxv5.New(dbPool)

	migrator, err := rivermigrate.New(driver, nil)
	if err == nil {
		_, _ = migrator.Migrate(ctx, rivermigrate.DirectionUp, &rivermigrate.MigrateOpts{})
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, NewUploadBatchWorker(dbPool, validator))

	riverClient, err := river.NewClient(driver, &river.Config{
		Workers: workers,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create river client: %w", err)
	}

	return riverClient, nil
}
