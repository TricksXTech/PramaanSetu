# Deterministic 5-Minute Presentation Script & Live Demo Guide

**Project**: APAAR Student Data Verification System for AICTE  
**Hackathon**: Smart India Hackathon (36-Hour Prototype)  
**Target Duration**: Exactly 5 Minutes (00:00 - 05:00)  
**Single Source of Truth**: Revised SIH Build Plan (Revision 2)

---

## Quick Reference: Pre-Seeded Accounts

All demo accounts share the uniform password **`Password@123`**. The presenter should click the quick-fill role credentials on the login screen (`/login`) to eliminate live typing.

| Role | Email | Password | Scope / Context |
|---|---|---|---|
| **AICTE Admin** | `admin@aicte.gov.in` | `Password@123` | National oversight, cross-institution reviews, aggregate rollups |
| **Institution Admin (DTU)** | `admin@dtu.ac.in` | `Password@123` | Delhi Technological University (`INST-AICTE-001`, Regular Cap: 2) |
| **Institution Admin (COEP)** | `admin@coep.ac.in` | `Password@123` | College of Engineering Pune (`INST-AICTE-002`, Regular Cap: 60) |
| **Student (Rohan Gupta)** | `student1@example.com` | `Password@123` | APAAR ID: `123456789012`, Enrolment: BTECH-CSE |
| **Verifier (TCS Campus HR)** | `verifier@tcs.com` | `Password@123` | External background check verifier |

---

## 5-Minute Timed Presentation Breakdown

### 00:00 - 01:00 &bull; Phase 1: Batch Ingestion & Row Outcomes (Rules V1–V6)

* **Goal**: Demonstrate high-speed CSV streaming, staging, and granular validation outcomes with plain-language explanations.
* **Presenter Action**:
  1. Open browser at `http://localhost:8080/login`.
  2. Click `admin@dtu.ac.in`, click **Sign In**.
  3. Land on **DTU Dashboard** (`/institution/dashboard`). Point out the **Approved Regular Intake Cap of 2 seats** for B.Tech CSE.
  4. Under **"Upload Admission Roster (CSV)"**, choose `scripts/fixtures/test_admissions.csv` (10 rows).
  5. Click **"Upload & Run Verification Pipeline"**.
  6. Observe HTMX progress bar advance rapidly to 100%. Click **"View Detailed Row Outcomes"**.
* **What to Point Out on Screen**:
  - **Granular Outcomes**: Total 10 rows processed &mdash; **1 Accepted**, **7 Flagged Held**, **1 Rejected Parse**, **1 Rejected Invalid**.
  - **Rule V1 (Syntax)**: Row 3 flagged for invalid 11-digit APAAR syntax (`apaar_id: "12345678901"`).
  - **Rule V3 (File Duplicate)**: Row 9 flagged as an exact in-file duplicate of Row 1.
  - **Rule V5 (Intake Cap Overflow)**: Rows 5, 6, 7, 8 held because DTU regular intake cap is 2 seats.
  - **Rule V6 (Policy Contradiction)**: Row 4 flagged for contradictory NRI + Foreign National status and Indian nationality.
  - **Zero Overwrite (Rule #1)**: Existing verified student identities remain untouched; no blind upserts.
* **Expected Numbers**: Accepted: 1, Flagged Held: 7, Rejected Parse: 1, Rejected Invalid: 1.
* **Fallback Plan**: If upload stalls, open terminal and run `curl -s http://localhost:8080/jobs` or click the pre-processed batch link in history table.

---

### 01:00 - 01:45 &bull; Phase 2: National AICTE Review & Fuzzy Matching (Rules V7, V8)

* **Goal**: Demonstrate regulatory cross-institution duplicate detection and fuzzy identity matching without exposing raw PII.
* **Presenter Action**:
  1. Click **Sign Out**, select `admin@aicte.gov.in`, click **Sign In**.
  2. In the navigation bar, click **"Review Queue"** (`/aicte/reviews`).
* **What to Point Out on Screen**:
  - **Cross-Institution Lead (Rule V7)**: Identifies a student enrolled full-time concurrently at DTU (Delhi) and COEP (Pune) in the same academic year (`2024-25`).
  - **Fuzzy Identity Candidate (Rule V8)**: Identifies high similarity (`score: 0.73 >= 0.65`) between `"Aarav Sharma"` (COEP) and `"Arav Sharma"` (DTU) sharing identical DOB and father's name.
  - **Privacy Preservation (Non-negotiable Rule #7)**: Personal identifiers are masked (`XXXX-XXXX-1234`). AICTE reviewers see institution names and evidence without full unmasked personal data.
  - **Audited Resolution**: Select decision **"Confirmed Different People (Spelling Variant)"**, enter note `"Verified via state domicile certificate"`, and click **"Record Determination"**.
  - Flag is updated to `resolved`, and an entry is immediately appended to the hash-chained audit log.
* **Fallback Plan**: If review queue is empty, trigger `curl -X POST http://localhost:8080/api/v1/trigger-leads` or reload `/aicte/reviews`.

---

### 01:45 - 02:30 &bull; Phase 3: Real-Time Rollups, Quota Gauge & Volume Anomaly (Rule V9)

* **Goal**: Show national aggregate rollups, approved quota adherence, and historical anomaly detection.
* **Presenter Action**:
  1. Click **"AICTE National Dashboard"** (`/aicte/dashboard`).
* **What to Point Out on Screen**:
  - **National Aggregate Cards**: Total Validated Admissions across India, Active Institutions, Open Leads.
  - **Intake vs Capacity Table**: Visual indicator shows institutions within approved intake vs those exceeding capacity.
  - **Volume Anomaly Detector (Rule V9)**: Compares current institution intake against historical multi-year mean and standard deviation. Flagged warning shown for abnormal surges (> 2 standard deviations).
  - **Zero Polling Lag**: Rollup tables updated transactionally upon row acceptance.
* **Fallback Plan**: If charts do not animate, refresh page (`Ctrl+R`); local vendored `chart.umd.min.js` renders statically.

---

### 02:30 - 04:00 &bull; Phase 4: Attestation, Ed25519 Signing, QR & Consent-Gated Verification

* **Goal**: Prove cryptographic authenticity and strict dynamic student consent enforcement.
* **Presenter Action**:
  1. Click **Sign Out**, select `admin@dtu.ac.in`, click **Sign In**.
  2. Navigate to **"Credentials Portal"** (`/institution/credentials`).
  3. Locate Rohan Gupta's accepted enrolment, click **"Attest & Issue Verifiable Credential"**.
  4. Point out the generated **Ed25519 Signature (64-byte hex)** and **Opaque QR Code**.
  5. Click **Sign Out**, select `verifier@tcs.com`, click **Sign In**.
  6. Navigate to `/verifier/verify`. Enter Rohan Gupta's Credential UUID, click **"Verify Credential"**.
  7. **Show Consent Gate**: Status displays **"Consent Required"** &mdash; Format Valid: YES, Signature Valid: YES, Registry Matched: YES, but **Permitted Fields: NONE (Zero Personal Data Revealed)**.
  8. Click **Sign Out**, select `student1@example.com` (Rohan Gupta), click **Sign In**.
  9. In **"Dynamic Consent Requests"**, click **"Grant Consent"** to TCS Campus HR for Purpose: *Employment Verification*, Permitted Fields: *Name, Program, Status*, Duration: *30 Days*.
  10. Switch back to `verifier@tcs.com`, re-run lookup.
  11. **Show Data Reveal**: Now reveals only consented fields (*Name: Rohan Gupta, Program: B.Tech CSE, Status: Attested*).
  12. Switch back to student, click **"Revoke Consent"**. Re-query as verifier &mdash; immediate denial with zero caching!
* **Fallback Plan**: If credential issuance was already done in seed, directly query UUID `e58ed763-928c-4155-bee9-fdbaaadc15f3`.

---

### 04:00 - 04:30 &bull; Phase 5: Printable Certificate, Webhook & Audit Verification

* **Goal**: Demonstrate physical-digital trust bridge and tamper-evident audit logs.
* **Presenter Action**:
  1. In student portal, click **"Printable Credential Certificate"** (`/credentials/:id/certificate`).
  2. Point out: Clean A4 portrait styling, institution seal, high-contrast typography, clear opaque QR. Press `Ctrl+P` to preview: nav bars and buttons vanish automatically.
  3. Click the Opaque QR URL (`/verify/opq_...`): Public landing page displays validity state without disclosing student profile.
  4. In a new tab, navigate to `http://localhost:8080/aicte/webhooks`.
  5. Show delivered **HMAC-SHA256 signed flag webhook** payload received by `/demo/webhook-receiver`.
  6. Navigate to `http://localhost:8080/aicte/audit/verify`.
  7. Click **"Verify Hash Chain"**: Confirms complete SHA-256 genesis-to-head tamper-evident audit trail with zero broken links.
* **Fallback Plan**: Open `http://localhost:8080/docs/api` to show the offline OpenAPI specification.

---

### 04:30 - 05:00 &bull; Phase 6: Performance Benchmark & Architectural Summary

* **Goal**: Reiterate adherence to fixed stack and proven scale metrics.
* **Presenter Talking Points**:
  - **Fixed Stack Integrity**: Pure Go binary + PostgreSQL 16. Zero Redis, zero Kafka, zero Python ML services.
  - **High-Throughput Ingestion**: Benchmarked at **$\ge 3,000$ rows/sec staging** and **$\ge 1,000$ rows/sec full pipeline validation**.
  - **Production Roadmap**: Designed to scale to 300 Million student records using PostgreSQL declarative partitioning and River job workers.
  - **Closing Statement**: *"The APAAR Student Data Verification System empowers AICTE with real-time regulatory oversight, prevents fraudulent double enrolments, protects institutional autonomy through tenant isolation, and places students in sovereign control of their verifiable academic credentials."*
* **Open for Jury Q&A**.

---

## Emergency Troubleshooting Cheatsheet

| Symptom | Cause | One-Line Fix Command |
|---|---|---|
| Database connection refused | Docker container paused | `docker-compose up -d db` |
| Seeding data needed | Fresh baseline desired | `go run ./cmd/seed/main.go` |
| Ingestion queue paused | River worker restarted | Automatic resume upon query; or restart `./bin/server` |
| Port 8080 already in use | Stale process | `Stop-Process -Name server -Force` (PowerShell) |
| Verification returns 401 | API key missing header | Add `-H "X-API-Key: <key>"` or login as verifier |
