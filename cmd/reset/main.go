package main

import (
	"context"
	"log"
	"os"
	"time"

	"apaar-verify/internal/database"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://apaar_user:apaar_secret@localhost:5432/apaar_verification?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Printf("[Reset] Connecting to database...")
	db, err := database.ConnectPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("[Reset] Connection failed: %v", err)
	}
	defer db.Close()

	truncateSQL := `
		TRUNCATE TABLE audit_events, audit_head, validation_flags, upload_rows, upload_jobs,
		               credentials, consents, enrolments, approved_intake,
		               api_keys, sessions, users, programs, institutions,
		               students, rollup_enrolment, institution_history CASCADE;
	`
	log.Printf("[Reset] Truncating all application tables...")
	if _, err := db.Exec(ctx, truncateSQL); err != nil {
		log.Fatalf("[Reset] Truncate failed: %v", err)
	}

	log.Printf("[Reset] Database reset cleanly.")
}

