package main

import (
	"context"
	"log"
	"os"
	"time"

	"apaar-verify/internal/database"
	"apaar-verify/internal/intake"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://apaar_user:apaar_secret@localhost:5432/apaar_verification?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("[Rollup Rebuild] Connecting to database...")
	db, err := database.ConnectPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("[Rollup Rebuild] Database connection error: %v", err)
	}
	defer db.Close()

	count, err := intake.RebuildRollups(ctx, db)
	if err != nil {
		log.Fatalf("[Rollup Rebuild] Error rebuilding rollups: %v", err)
	}

	log.Printf("[Rollup Rebuild] Successfully reconstructed %d rollup cohort rows.", count)
}

