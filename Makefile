.PHONY: up down build seed reset test lint rollup-rebuild smoke bench help

help:
	@echo "Available targets:"
	@echo "  make up             - Build and start all services via Docker Compose"
	@echo "  make down           - Stop and remove Docker Compose containers and volumes"
	@echo "  make build          - Build local Go binaries for server, seed, reset, and utilities"
	@echo "  make seed           - Populate database with deterministic initial data"
	@echo "  make reset          - Truncate application tables and re-run seed"
	@echo "  make rollup-rebuild - Deterministically recompute rollups from enrolments"
	@echo "  make test           - Run all unit and boundary isolation tests"
	@echo "  make lint           - Run go vet against all packages"
	@echo "  make smoke          - Run end-to-end smoke test suite across all 14 demo steps"
	@echo "  make bench          - Run performance benchmark harness and generate reports"

up:
	docker compose up --build

down:
	docker compose down -v

build:
	go build -o bin/server ./cmd/server
	go build -o bin/seed ./cmd/seed
	go build -o bin/reset ./cmd/reset
	go build -o bin/rollup-rebuild ./cmd/rollup_rebuild
	go build -o bin/smoke ./cmd/smoke
	go build -o bin/benchmark ./cmd/benchmark

seed:
	go run ./cmd/seed/main.go

reset:
	go run ./cmd/reset/main.go
	go run ./cmd/seed/main.go

rollup-rebuild:
	go run ./cmd/rollup_rebuild/main.go

test:
	go test -v ./...

lint:
	go vet ./...

smoke:
	go run ./cmd/smoke/main.go

bench:
	go run ./cmd/benchmark/main.go


