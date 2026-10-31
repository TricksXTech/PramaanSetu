// Run with: go run ./scripts/seed.go
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

	db, err := database.ConnectPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	log.Println("Run `go run ./cmd/seed/main.go` for the full repeatable seed script.")
}
