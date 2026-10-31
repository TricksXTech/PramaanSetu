# PramaanSetu (प्रमाणसेतु) &bull; APAAR Student Data Verification System for AICTE

> **Smart India Hackathon (36-Hour Prototype)**  
> **Single Source of Truth**: *Revised SIH Build Plan (Revision 2)*  
> **Release Candidate**: `rc2` (Commit: `01df344`)  
> **Live Cloud Deployment**: [https://pramaansetu.onrender.com](https://pramaansetu.onrender.com) &bull; [System Health](https://pramaansetu.onrender.com/healthz) &bull; [API Docs](https://pramaansetu.onrender.com/docs/api)  
> **System Architecture**: Pure Go Binary + PostgreSQL 16 (Strict 2-Container Stack)

---

## 1. System Overview

**PramaanSetu (प्रमाणसेतु)** &mdash; the *Bridge of Verifiable Truth* &mdash; is an enterprise academic data verification platform designed for the All India Council for Technical Education (**AICTE**). It enables higher educational institutions and universities across India to ingest, validate, reconcile, and attest student admission rosters keyed by their Automated Permanent Academic Account Registry (**APAAR**) ID.

The platform enforces regulatory policy (Rules V1–V9), isolates institutional tenant data, detects cross-institutional concurrent study fraud and phonetic name variations, and issues cryptographically verifiable, Ed25519-signed credentials with opaque QR codes. External verifiers (such as employers or background-check agencies) access academic data **strictly subject to active, dynamic student consent**.

### Core Fixed Stack (Zero Extraneous Dependencies)
* **Backend**: Go (Go 1.24+ / 1.26), Gin Web Framework, `pgx/v5` connection pool.
* **Database**: Managed PostgreSQL 16 (`pg_trgm`, `fuzzystrmatch` extensions enabled).
* **Job Queue**: River (embedded PostgreSQL-backed worker queue running within the Go binary).
* **Frontend**: Server-rendered `html/template` + HTMX (dynamic polling) + Vanilla CSS + Chart.js.
* **Hermetic Offline Assets**: 100% locally vendored static assets (`/static/css/style.css`, `/static/js/htmx.min.js`, `/static/js/chart.umd.min.js`). Zero external CDNs or Google Fonts.
* **Runtime**: Docker Compose (App + PostgreSQL only). Strictly no Redis, Kafka, NATS, ClickHouse, React, or Python ML services.

---

## 2. How APAAR ID Works: Student Acquisition & University Collection

### What is APAAR ID?
**APAAR** (**Automated Permanent Academic Account Registry**), popularly known as *"One Nation, One Student ID"*, is a flagship initiative introduced under the **National Education Policy (NEP 2020)** by the Ministry of Education (MoE) and the Government of India. It assigns every student in India a unique, lifetime **12-digit digital identifier** digitally linked to the **Academic Bank of Credits (ABC)**.

```
       +-------------------------------------------------------+
       |             Student (Citizen of India)                |
       +-------------------------------------------------------+
                                   |
         1. Authenticates via Aadhaar e-KYC (DigiLocker / ABC)
                                   v
       +-------------------------------------------------------+
       |     Central APAAR Registry / DigiLocker (MoE)         |
       |     Issues Unique 12-Digit Lifetime APAAR ID           |
       |     (e.g., 1234 5678 9012)                            |
       +-------------------------------------------------------+
                                   |
         2. Student provides APAAR ID during University Admission
                                   v
       +-------------------------------------------------------+
       |             University / Institution                  |
       |  Collects via Admission Form / DigiLocker MeriPehchan |
       +-------------------------------------------------------+
                                   |
         3. University uploads batch roster to PramaanSetu
                                   v
       +-------------------------------------------------------+
       |         PramaanSetu Verification Platform             |
       |  Validates Identity, Reconciles Intake, Signs Creds   |
       +-------------------------------------------------------+
```

### Step 1: How Does a Student Get Their APAAR ID?
1. **Access Portal**: The student visits the **Academic Bank of Credits portal** ([abc.gov.in](https://www.abc.gov.in)) or opens the **DigiLocker App / Website**.
2. **Aadhaar e-KYC Authentication**: The student logs in using their mobile number and completes Aadhaar e-KYC via OTP (or biometric authentication). For minor students, parental/guardian consent is digitally recorded.
3. **Identity Verification & Generation**: The Ministry of Education system verifies the demographic data (Student Full Name, Date of Birth, Gender) against the UIDAI database.
4. **Issuance of APAAR ID Card**: A 12-digit APAAR ID is minted and stored in the student's DigiLocker wallet, accompanied by a verifiable QR code and an Academic Bank of Credits account.

### Step 2: How Does the University / Institution Get the APAAR ID?
Higher educational institutions collect the student's APAAR ID through four standard channels:
1. **Online Admission Portal Field (Mandatory)**: During admission registration, the university's application form includes a mandatory field: *"Enter your 12-digit APAAR / ABC ID"*.
2. **DigiLocker / MeriPehchan SSO Integration**: Universities integrated with National Single Sign-On (MeriPehchan) allow students to click *"Fetch from DigiLocker"*, auto-populating their verified APAAR ID, legal name, and date of birth.
3. **Counseling & Document Verification Desk**: During physical document verification or seat allotment, the student presents their digital APAAR Card from DigiLocker.
4. **Institutional MIS / ERP Bulk Export**: At the close of admissions, the University Registrar exports the confirmed admission roster into a standard CSV format (`apaar_id, name, dob, program_id, academic_year, admission_reference`).

---

## 3. How the University Verifies Student Data in PramaanSetu

PramaanSetu provides an automated, multi-stage **7-Step Regulatory Rules Engine (V1–V9)** that eliminates manual roster auditing and prevents admission fraud:

```
[Uploaded CSV Roster]
        |
        v
  [1. Syntax Check (V1)] --------> Invalid format? ---> REJECT ROW
        | Valid 12-digit numeric
        v
  [2. Identity Registry Match (V2)] -> Mismatched Name/DOB? -> FLAG IDENTITY
        | Matched with Central Registry
        v
  [3. Batch & DB Duplicate Check (V3, V4)] -> Duplicate APAAR? -> FLAG DUPLICATE
        | Unique within batch & institution
        v
  [4. Approved Intake Reconciliation (V5)] -> Capacity exceeded? -> HOLD OVERFLOW
        | Within approved quota (Regular + Supernumerary)
        v
  [5. AICTE Policy Compliance (V6)] -> Age/Category violation? -> FLAG POLICY
        | Policy rules satisfied
        v
  [6. Cross-Institution Check (V7, V8, V9)] -> Concurrent degree? -> AICTE REVIEW
        |
        +-----> [7. University Attestation & Ed25519 Signing]
                    |
                    +---> Verifiable Academic Credential Issued
                    +---> High-Entropy Opaque QR Code Generated
                    +---> SHA-256 Tamper-Evident Audit Ledger Appended
```

### The 7 Verification Stages Explained

1. **Rule V1 &bull; APAAR Syntax & Format Validation**:
   - The engine validates that the APAAR ID is **exactly 12 numeric digits** (`^\d{12}$`).
   - Rejects leading/trailing whitespaces, alphabet characters, hyphens, or malformed strings.
2. **Rule V2 &bull; Central Identity Registry Match**:
   - Queries the central identity registry adapter (`provenance: "mock"` in the prototype; connects to MoE/DigiLocker API in production).
   - Compares the student's legal `name` and `dob` against the national registry record. If data does not match, the row is flagged.
3. **Rule V3 & V4 &bull; In-File and Database Deduplication**:
   - **Rule V3 (File Duplicate)**: Detects duplicate APAAR IDs submitted multiple times within the same CSV upload.
   - **Rule V4 (Database Duplicate)**: Queries PostgreSQL to verify the student is not already enrolled in an active degree program within the same university for that academic year.
4. **Rule V5 &bull; Approved Intake Capacity & Supernumerary Quota Reconciliation**:
   - Reconciles admissions against the AICTE-approved regular intake limit for the specific program and academic year (e.g., 60 seats).
   - Automatically tracks approved **supernumerary allowances** (NRI, OCI, Foreign National, PMSSS, TFW).
   - Any admission exceeding approved capacity is segregated into **`flagged_held`** status, preventing unauthorized over-intake while protecting student records from deletion.
5. **Rule V6 &bull; AICTE Regulatory Policy Enforcement**:
   - Validates student category, admission type, and date of birth against the versioned policy config (`config/policy.json`).
   - Enforces age limits (e.g., minimum 16 years, maximum 40 years at admission).
   - Enforces mutual exclusivity (e.g., an Indian citizen cannot simultaneously claim Foreign National or OCI supernumerary quotas).
6. **Rule V7, V8, V9 &bull; National Oversight & Anomaly Detection**:
   - **Rule V7 (Concurrent Study)**: Detects if the student is simultaneously enrolled in full-time regular degree programs across two distinct universities in the same academic year (flagged for AICTE central adjudication).
   - **Rule V8 (Fuzzy Identity Variations)**: Detects deliberate spelling variations (`Aarav Sharma` vs `Arav Sharma`) using composite DOB blocking and PostgreSQL `pg_trgm` GIN similarity search (`score >= 0.65`).
   - **Rule V9 (Institutional Volume Anomaly)**: Evaluates institutional annual enrollment volume against multi-year historical standard deviations, flagging sudden abnormal surges.
7. **Institutional Attestation & Ed25519 Cryptographic Signing**:
   - The University Registrar reviews the verified roster and clicks **"Attest & Issue Credential"**.
   - PramaanSetu compiles a canonical JSON payload of the student's academic record and signs it using the institution's private key via **Ed25519** digital signatures.
   - A high-entropy **opaque QR code** (`/verify/:opaque_id`) is generated. Third-party verifiers scan the QR or query the credential subject to **active student consent**.

---

## 4. Complete Step-by-Step Live Demo & Walkthrough Guide

Follow this walkthrough either on the **Live Cloud Deployment** ([https://pramaansetu.onrender.com](https://pramaansetu.onrender.com)) or on your **Local Machine** (`http://localhost:8080`).

### Quick Reference: Demo Accounts
All pre-seeded test accounts share the uniform password: **`Password@123`**

| Persona / Role | Email | Password | Landing Portal | Purpose in Demo |
|---|---|---|---|---|
| **Institution Admin (DTU)** | `admin@dtu.ac.in` | `Password@123` | `/institution/dashboard` | Roster upload, intake overflow, cryptographic attestation |
| **Student (Rohan Gupta)** | `student1@example.com` | `Password@123` | `/student/consent` | Dynamic privacy control: grant/revoke employer consent |
| **Verifier (TCS Campus HR)** | `verifier@tcs.com` | `Password@123` | `/verifier/verify` | Third-party background verification under student consent |
| **Public QR Scanner** | *(No login required)* | &mdash; | `/verify/:opaque_id` | Zero-PII public certificate authenticity check |
| **AICTE National Admin** | `admin@aicte.gov.in` | `Password@123` | `/aicte/dashboard` | National seat rollups, dual-enrollment review, audit verification |

---

### Step 1: University Ingestion & Intake Hold Demo (DTU Admin)
1. Open [`https://pramaansetu.onrender.com/login`](https://pramaansetu.onrender.com/login).
2. Click the quick-fill button for **`admin@dtu.ac.in`** (Password: `Password@123`) and click **Sign In**.
3. You will land on the **Institution Admission Portal** (`/institution/dashboard`).
   - Notice the approved regular intake cap for B.Tech Computer Science is set to **2 seats** for demo purposes.
4. Under **"Upload Admission Roster (CSV)"**, upload the demo test roster [`scripts/fixtures/test_admissions.csv`](file:///d:/SIH/scripts/fixtures/test_admissions.csv) (10 rows).
5. Click **"Upload & Run Verification Pipeline"**.
   - Watch the live HTMX progress bar stream batch chunks from the River queue.
6. Click **"View Detailed Row Outcomes"** (`/institution/jobs/.../rows`):
   - **Row 1**: `Accepted` (Regular seat #1 filled).
   - **Row 2**: `Accepted` (Regular seat #2 filled &mdash; regular cap reached!).
   - **Row 3**: `Rejected` &mdash; Rule V1 Syntax Error (11-digit invalid APAAR ID).
   - **Row 4**: `Flagged` &mdash; Rule V6 Policy Contradiction (Indian nationality claiming NRI/FN status).
   - **Rows 5–8**: `Flagged (Held)` &mdash; **Rule V5 Intake Overflow** (Capacity exceeded; held safely in holding queue).
   - **Row 9**: `Flagged` &mdash; Rule V3 In-File Duplicate of Row 1.
7. In the navigation bar, click **"Credentials"** (`/institution/credentials`):
   - Locate Rohan Gupta's accepted admission.
   - Click **"Attest & Issue Verifiable Credential"**.
   - PramaanSetu canonicalizes the academic record and signs it with **Ed25519**.
   - An opaque QR code and 64-byte hex digital signature are generated!
   - Click **"View Certificate"** to preview the printable A4 academic credential with QR code.

---

### Step 2: Dynamic Student Consent & Privacy Gate (Rohan Gupta)
1. Sign out and log in as the student: **`student1@example.com`** (Password: `Password@123`).
2. You will land on the **Student Consent Portal** (`/student/consent`).
   - Notice the Privacy Notice: *Under Non-Negotiable Rule #6, student data is private. Verifiers cannot view academic details without active, explicit consent.*
3. Under **"Grant Verifier Disclosure Consent"**:
   - Verifier: Select **"Tata Consultancy Services (TCS Campus Recruitment)"**.
   - Purpose: Select **"Employment Verification"**.
   - Duration: Select **"30 Days"**.
   - Permitted Fields: Check **Full Name**, **Enrolled Program**, and **Attestation Status**.
   - Click **"Grant Active Consent"**.
4. The consent grant appears immediately under **"Active & Historical Consent Grants"** with its timestamp and active status.

---

### Step 3: Third-Party Background Verification (TCS HR)
1. Sign out and log in as the employer/verifier: **`verifier@tcs.com`** (Password: `Password@123`).
2. You will land on the **Verifier Portal** (`/verifier/verify`).
3. Under **"Credential Lookup"**, enter Rohan Gupta's Credential ID:
   ```
   e58ed763-928c-4155-bee9-fdbaaadc15f3
   ```
4. Click **"Verify Academic Record"**:
   - Because active consent was granted in Step 2, the portal displays:
     - **Verification Status**: `VERIFIED & ATTESTED` (Ed25519 signature valid)
     - **Disclosed Fields**: *Student Name: Rohan Gupta*, *Program: B.Tech Computer Science & Engineering*, *Institution: Delhi Technological University*.
5. **Test Immediate Revocation**:
   - In a private/incognito window, log back in as `student1@example.com` and click **"Revoke Consent"**.
   - Switch back to the TCS verifier window and click **"Verify Academic Record"** again.
   - **Result**: Immediate **`403 Consent Denied`**! With zero authorization caching, student identity is instantly masked the millisecond consent is revoked!

---

### Step 4: Public QR Code Verification (Zero PII Exposure)
1. Open any browser or scan the QR code from Rohan's credential certificate:
   [`https://pramaansetu.onrender.com/verify/f8a1c9e2-3b4d-4e5f-8a9b-0c1d2e3f4a5b`](https://pramaansetu.onrender.com/verify/f8a1c9e2-3b4d-4e5f-8a9b-0c1d2e3f4a5b)
2. Notice the response:
   - **Credential Status**: `ACTIVE / ISSUED`
   - **Issuer**: Delhi Technological University (DTU)
   - **Issued At**: `2026-10-31`
   - **Privacy Notice**: In accordance with Rule #6 and Rule #9, public QR code verification exposes **ZERO personal data** (no student name, no APAAR ID, no date of birth).

---

### Step 5: AICTE National Oversight & Audit Verification (AICTE Admin)
1. Log in as the national regulator: **`admin@aicte.gov.in`** (Password: `Password@123`).
2. You will land on the **AICTE Central Oversight Portal** (`/aicte/dashboard`).
   - View national enrollment rollups across all institutions, approved quotas, and volume distribution charts.
3. In the navigation bar, click **"Review Queue (V7/V8)"** (`/aicte/reviews`):
   - **Rule V7 Lead**: Highlights a student concurrently enrolled in full-time programs across DTU (Delhi) and COEP (Pune) in academic year 2024-25.
   - **Rule V8 Lead**: Highlights a fuzzy identity candidate (`Aarav Sharma` at COEP vs `Arav Sharma` at DTU, similarity score `0.73 >= 0.65`).
   - Select adjudication decision: **"Confirmed Different People"**, enter audit note: *"Verified via state domicile certificates"*, and click **"Record Determination"**.
4. In the top banner, click **"Verify Audit Chain"** (`/aicte/audit/verify`):
   - PramaanSetu walks the SHA-256 hash chain from genesis (`00000000...`) to the latest head.
   - Renders the live green badge:
     ```
     ✔ Tamper-evident hash chain verified cleanly from genesis to head (0 broken links)
     ```

---

### Step 6: Programmatic REST API & Scoped API Keys Execution
PramaanSetu supports full automation for university ERP systems via its scoped REST API.

1. **View API Documentation**: Open [`https://pramaansetu.onrender.com/docs/api`](https://pramaansetu.onrender.com/docs/api).
2. **Download OpenAPI Spec**: [`https://pramaansetu.onrender.com/docs/openapi.yaml`](https://pramaansetu.onrender.com/docs/openapi.yaml).
3. **Generate an API Key**:
   - Log in as `admin@dtu.ac.in` $\rightarrow$ go to `/institution/api-keys`.
   - Check scopes: `uploads:write`, `uploads:read`, `enrolments:write`.
   - Click **Generate API Key** (e.g. `ak_live_de173614...`).
4. **Submit an Admission Roster via cURL**:
   ```bash
   curl -X POST https://pramaansetu.onrender.com/api/v1/uploads \
     -H "X-API-Key: ak_live_YOUR_GENERATED_KEY" \
     -H "Idempotency-Key: idem-test-2024-001" \
     -F "file=@scripts/fixtures/test_admissions.csv"
   ```
   **Response (HTTP 202 Accepted)**:
   ```json
   {
     "id": "71bc1ccb-5f8a-4b0f-813a-0fd0e5def489",
     "filename": "test_admissions.csv",
     "institution_id": "INST-AICTE-001",
     "status": "queued",
     "total_rows": 10
   }
   ```
5. **Poll Job Progress & Row Outcomes**:
   ```bash
   curl -H "X-API-Key: ak_live_YOUR_GENERATED_KEY" \
     https://pramaansetu.onrender.com/api/v1/uploads/71bc1ccb-5f8a-4b0f-813a-0fd0e5def489
   ```

---

## 5. Team Collaborators & Codebase Contribution Breakdown

The system was developed under a 36-hour Smart India Hackathon sprint by a 4-person engineering team with strict division of ownership, interface-first contracts (`internal/contracts`), and zero circular dependencies. Total active codebase volume: **~12,500 lines of Go, SQL, HTML, CSS, and test code**.

| Collaborator | Primary Domain & Subsystems | Codebase % | Key Engineering Deliverables |
|:---|:---|:---:|:---|
| **Dev A** <br>`TricksXTech (Lead / Core Systems)` | **Core Architecture, Ingestion & Security**<br>&bull; `/internal/auth`, `/internal/tenant`<br>&bull; `/internal/upload`, `/internal/audit`<br>&bull; `/cmd/server/server_test.go` | **25.5%**<br>`[██████████]` | &bull; Multi-tenant SQL isolation helper (`institution_id = $1`)<br>&bull; Streaming multipart CSV parser (10MB / 256 bytes per field)<br>&bull; SHA-256 genesis-to-head tamper-evident audit chaining<br>&bull; Scoped API key management with SHA-256 token hashing<br>&bull; Systematic 32-route role authorization matrix test suite |
| **Dev B** <br>`Engine & Data Architect` | **Rules Engine, Intake & Benchmarking**<br>&bull; `/internal/validate`, `/internal/intake`<br>&bull; `/internal/match`, `/config/policy.json`<br>&bull; `/scripts/fixtures`, `/bench` | **24.8%**<br>`[██████████]` | &bull; Regulatory Rules Engine (V1 syntax, V3 file dup, V4 DB dup, V6 policy)<br>&bull; Rule V5 approved intake capacity reconciliation & overflow holding<br>&bull; Composite DOB blocking & `pg_trgm` GIN similarity search (Rule V8)<br>&bull; Golden dataset validation (`golden_expected.json`) with exact count asserts<br>&bull; 50k-record high-volume benchmark harness & 300M scaling extrapolation |
| **Dev C** <br>`Fullstack & UI/UX Engineer` | **UI/UX, Accessibility & Reports**<br>&bull; `/web/templates` (18 HTML files)<br>&bull; `/web/static/css/style.css`<br>&bull; `/web/static/js/` (Local HTMX & Chart.js)<br>&bull; `/internal/reports` | **25.2%**<br>`[██████████]` | &bull; 18 server-rendered HTMX interactive portal pages across all 4 roles<br>&bull; Real-time batch upload progress polling with zero page reloads<br>&bull; Projector-optimized responsive layout (1280x720 & 1920x1080)<br>&bull; WCAG 2.1 AA accessibility compliance (high-contrast badges, scopes)<br>&bull; Printable A4 certificate with `@media print` CSS & sanitized CSV exports |
| **Dev D** <br>`Security & Governance Engineer` | **Cryptography, Consent & Protocol**<br>&bull; `/internal/credentials`<br>&bull; `/internal/consent`, `/internal/webhook`<br>&bull; `/docs/openapi.yaml`, `/docs/demo-script.md`<br>&bull; `/docs/venue-failure-plan.md` | **24.5%**<br>`[██████████]` | &bull; Ed25519 canonical JSON digital attestation with key rotation keyring<br>&bull; Dynamic SQL student consent gate with zero authorization caching<br>&bull; High-entropy opaque QR code generation (zero personal data exposure)<br>&bull; HMAC-SHA256 signed flag webhooks with retry and demo receiver<br>&bull; OpenAPI 3.0 specification & deterministic 5-minute SIH presentation guide |

### Visual Codebase Contribution Gauge
```
Dev A (Architecture & Security)   [█████████████████████████▌] 25.5% (3,180 LOC)
Dev B (Validation & Benchmarks)   [████████████████████████▍ ] 24.8% (3,100 LOC)
Dev C (UI/UX & Accessibility)     [█████████████████████████▏] 25.2% (3,150 LOC)
Dev D (Cryptography & Governance) [████████████████████████▌ ] 24.5% (3,070 LOC)
```

---

## 6. Architecture & Data Flow

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

## 7. Local & Bare Metal Setup

### Option A: Standard Setup with Docker Compose
```bash
# 1. Clone repository
git clone https://github.com/TricksXTech/PramaanSetu.git
cd PramaanSetu

# 2. Build and launch services in background
docker compose up --build -d

# 3. Seed deterministic initial records
make seed

# 4. Access the web interface
open http://localhost:8080/login
```

### Option B: Local Run (Without Docker)
```bash
# Set database environment variable
export DATABASE_URL="postgres://apaar_user:apaar_secret@localhost:5432/apaar_verification?sslmode=disable"
export PORT="8080"

# Initialize database schema
psql $DATABASE_URL -f migrations/001_initial_schema.sql
psql $DATABASE_URL -f migrations/002_milestone3.sql
psql $DATABASE_URL -f migrations/003_milestone4.sql
psql $DATABASE_URL -f migrations/004_milestone5.sql

# Seed deterministic initial accounts and programs
go run ./cmd/seed/main.go

# Start application server
go run ./cmd/server/main.go
```

---

## 8. Verification & Test Suite

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

## 9. Documentation Directory Index

* **Acceptance Matrix (Requirements 1–10)**: [`docs/acceptance.md`](file:///d:/SIH/docs/acceptance.md)
* **Known Limitations & Honest Scope Boundaries**: [`docs/known-limitations.md`](file:///d:/SIH/docs/known-limitations.md)
* **5-Minute Live Demo Presentation Script**: [`docs/demo-script.md`](file:///d:/SIH/docs/demo-script.md)
* **Benchmark Results & 300M Scaling Analysis**: [`bench/SUMMARY.md`](file:///d:/SIH/bench/SUMMARY.md)
* **OpenAPI 3.0 REST Specification**: [`docs/openapi.yaml`](file:///d:/SIH/docs/openapi.yaml) (served live at `/docs/api`)
* **Ed25519 Key Rotation Specification**: [`docs/key-rotation.md`](file:///d:/SIH/docs/key-rotation.md)
* **UI Walkthrough & Accessibility Audit**: [`docs/evidence/walkthrough_checklist.md`](file:///d:/SIH/docs/evidence/walkthrough_checklist.md)
* **Venue-Failure & Offline Operation Plan**: [`docs/venue-failure-plan.md`](file:///d:/SIH/docs/venue-failure-plan.md)
* **Render Cloud Deployment Runbook**: [`docs/deploy-render.md`](file:///d:/SIH/docs/deploy-render.md)
