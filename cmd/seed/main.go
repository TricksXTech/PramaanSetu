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

	log.Printf("[Seed] Connecting to database...")
	db, err := database.ConnectPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("[Seed] Failed to connect: %v", err)
	}
	defer db.Close()

	if err := database.RunSeed(ctx, db); err != nil {
		log.Fatalf("[Seed] Error executing seed: %v", err)
	}

	log.Println("[Seed] Repeatable database seeding completed successfully.")
}
