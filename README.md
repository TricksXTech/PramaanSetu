# APAAR Student Data Verification System for AICTE

> **Smart India Hackathon (36-Hour Prototype)**  
> **Single Source of Truth**: *Revised SIH Build Plan (Revision 2)*  
> **Release Candidate**: `rc1` (Commit: `2df1253`)  
> **System Architecture**: Pure Go Binary + PostgreSQL 16 (Strict 2-Container Stack)

---

## 1. System Overview

The **APAAR Student Data Verification System** enables higher education institutions across India to upload, validate, and attest student admission rosters keyed by their Automated Permanent Academic Account Registry (**APAAR**) ID. The system enforces regulatory policy (Rules V1–V9), isolates institutional tenant data, detects cross-institutional concurrent study leads and spelling variations, and issues cryptographically verifiable, Ed25519-signed credentials with opaque QR codes. Verifiers access student data **strictly subject to active, dynamic student consent**.

### Core Fixed Stack (Zero Extraneous Dependencies)
* **Backend**: Go (Go 1.24+ / 1.26), Gin Web Framework, `pgx/v5` connection pool.
* **Database**: PostgreSQL 16 (`pg_trgm`, `fuzzystrmatch` extensions).
* **Job Queue**: River (embedded PostgreSQL-backed worker queue running within the Go binary).
* **Frontend**: Server-rendered `html/template` + HTMX (dynamic polling) + Vanilla CSS + Chart.js.
* **Hermetic Offline Assets**: 100% locally vendored static assets (`/static/css/style.css`, `/static/js/htmx.min.js`, `/static/js/chart.umd.min.js`). Zero external CDNs or Google Fonts.
* **Runtime**: Docker Compose (App + PostgreSQL only). Strictly no Redis, Kafka, NATS, ClickHouse, React, or Python ML services.

---

## 2. Architecture & Data Flow

```mermaid
flowchart TD
    subgraph Client["Web Browser & REST API Clients"]
        InstAdmin["Institution Admin<br>(CSV / REST API)"]
        AICTEAdmin["AICTE Admin<br>(National Oversight)"]
        Student["Student<br>(Consent Portal)"]
        Verifier["Verifier / Employer<br>(Consent Gate)"]
    end

    subgraph GoServer["Go Application Binary (Single Process)"]
        Gin["Gin Web Router & Session/API Auth"]
        Engine["Rules Engine (V1 - V9)"]
        RiverWorker["River Worker Queue (Batch Chunks)"]
        Signer["Ed25519 Digital Signer"]
        Auditor["Hash-Chained Auditor (SHA-256)"]
        Rollup["Real-Time Rollup Aggregator"]
    end

    subgraph Database["PostgreSQL 16 Engine"]
        T_Students[("students (Identity Only)")]
        T_Enrolments[("enrolments (Tenant Data)")]
        T_Credentials[("credentials (Attested Payloads)")]
        T_Consents[("consents (Dynamic Grants)")]
        T_Audit[("audit_events (Append-Only)")]
        T_Rollup[("rollup_enrolment (Aggregates)")]
    end

    InstAdmin -->|"Upload CSV / API Key"| Gin
    AICTEAdmin -->|"Review Leads / Anomaly"| Gin
    Student -->|"Grant / Revoke Consent"| Gin
    Verifier -->|"Query Credential UUID"| Gin

    Gin --> RiverWorker
    RiverWorker --> Engine
    Engine -->|"V1-V6 Validation"| T_Enrolments
    Engine -->|"Cross-Inst Lead (V7) / Fuzzy (V8)"| AICTEAdmin
    Engine -->|"Transactional Update"| Rollup
    Rollup --> T_Rollup

    Gin -->|"Attest & Sign"| Signer
    Signer -->|"Store Canonical Signature"| T_Credentials
    Gin -->|"Dynamic Consent Check"| T_Consents
    Gin -->|"Append Chain Link"| Auditor
    Auditor --> T_Audit
```

---

## 3. Quick Start (Clean Machine)

### Prerequisites
* **Docker & Docker Compose** (Recommended for Linux / Mac / Windows with Docker Desktop) **OR**
* **Go 1.24+** and **PostgreSQL 16** installed locally.

### Standard Setup with Docker Compose
```bash
# 1. Clone repository
git clone https://github.com/aicte-apaar/apaar-verify.git
cd apaar-verify

# 2. Build and launch services in background
docker compose up --build -d

# 3. Seed deterministic initial records
make seed

# 4. Access the web interface
open http://localhost:8080/login
```

### Local Setup (Without Docker)
```bash
# Set database environment variable
export DATABASE_URL="postgres://apaar_user:apaar_secret@localhost:5432/apaar_verification?sslmode=disable"
export PORT="8080"

# Initialize database schema
psql $DATABASE_URL -f migrations/001_initial_schema.sql

# Seed deterministic initial accounts and programs
go run ./cmd/seed/main.go

# Start application server
go run ./cmd/server/main.go
```

---

## 4. Pre-Seeded Demonstration Accounts

All demo accounts share the uniform password **`Password@123`**:

| Role | Email Address | Password | Description / Context |
|---|---|---|---|
| **AICTE National Admin** | `admin@aicte.gov.in` | `Password@123` | National rollup dashboard, review queue, audit verify |
| **Institution Admin (DTU)** | `admin@dtu.ac.in` | `Password@123` | Delhi Tech University (`INST-AICTE-001`), Regular Cap: 2 |
| **Institution Admin (COEP)** | `admin@coep.ac.in` | `Password@123` | College of Engg Pune (`INST-AICTE-002`), Regular Cap: 60 |
| **Student (Rohan Gupta)** | `student1@example.com` | `Password@123` | APAAR ID: `123456789012`, Enrolment: B.Tech CSE |
| **Student (Aarav Sharma)** | `student2@example.com` | `Password@123` | APAAR ID: `987654321098`, Enrolment: B.Tech CSE |
| **Verifier (TCS Campus HR)** | `verifier@tcs.com` | `Password@123` | Third-party employer background verification |

---

## 5. Verification & Testing

### Run All Unit & Integration Tests
```bash
# Run full suite (cross-tenant, role matrix, golden fixture, intake limits)
make test
# OR
go test -v ./...
```

### Run End-to-End Smoke Test Suite
```bash
# Executes all 14 end-to-end verification steps deterministically
make smoke
# OR
go run ./cmd/smoke/main.go
```

### Reset & Reseed Database
```bash
# Truncates all tables and re-seeds deterministic state (< 5 seconds)
make reset
```

### Run High-Volume Benchmarks
```bash
# Runs streaming CSV parser, validation engine, trigram blocking, and verifier gate
make bench
# OR
go run ./cmd/benchmark/main.go
```

---

## 6. Regulatory Rules Implemented (V1 – V9)

* **Rule V1 (Syntax)**: Validates exactly 12 numeric digits without letters, symbols, or leading spaces.
* **Rule V2 (Registry Match)**: Queries APAAR registry adapter (`provenance: "mock"`).
* **Rule V3 (File Duplicate)**: Rejects duplicate APAAR IDs within the same uploaded CSV file.
* **Rule V4 (Database Duplicate)**: Checks existing enrolments in the same institution and academic year.
* **Rule V5 (Approved Intake Reconciliation)**: Evaluates regular intake cap vs accepted count; overflow held as `flagged_held`.
* **Rule V6 (Policy Contradictions)**: Checks mutual exclusivity of NRI / Foreign National / OCI status and age window (16–40).
* **Rule V7 (Cross-Institution Concurrent Study)**: Flags student concurrently enrolled in full-time programs across two institutions.
* **Rule V8 (Fuzzy Identity Variations)**: Identifies near-duplicate identities using composite DOB blocking and GIN trigram similarity (`score >= 0.65`).
* **Rule V9 (Institutional Volume Anomaly)**: Flags abnormal annual enrolment surges (> 2 standard deviations above historical mean).

---

## 7. Documentation Directory

* **Acceptance Matrix (Requirements 1–10)**: [`docs/acceptance.md`](file:///d:/SIH/docs/acceptance.md)
* **Known Limitations & Honest Scope Boundaries**: [`docs/known-limitations.md`](file:///d:/SIH/docs/known-limitations.md)
* **5-Minute Live Demo Presentation Script**: [`docs/demo-script.md`](file:///d:/SIH/docs/demo-script.md)
* **Benchmark Results & 300M Scaling Analysis**: [`bench/SUMMARY.md`](file:///d:/SIH/bench/SUMMARY.md)
* **OpenAPI 3.0 REST Specification**: [`docs/openapi.yaml`](file:///d:/SIH/docs/openapi.yaml) (served live at `/docs/api`)
* **Ed25519 Key Rotation Specification**: [`docs/key-rotation.md`](file:///d:/SIH/docs/key-rotation.md)
* **UI Walkthrough & Accessibility Audit**: [`docs/evidence/walkthrough_checklist.md`](file:///d:/SIH/docs/evidence/walkthrough_checklist.md)
* **Venue-Failure & Offline Operation Plan**: [`docs/venue-failure-plan.md`](file:///d:/SIH/docs/venue-failure-plan.md)
