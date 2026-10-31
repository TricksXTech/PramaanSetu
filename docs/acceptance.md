# APAAR Student Data Verification System - Acceptance Matrix

Single Source of Truth: **Revised SIH Build Plan (Revision 2)**  
Status Date: **2026-10-31**  
Milestone: **Milestone 6 (Integration & Acceptance)**  

---

## 1. Ten-Requirement Acceptance Map (Section 12)

| # | Deliverable & Acceptance Test | How to Run (Command or Click Path) | Owner | Status | Evidence | Notes |
|:---:|---|---|:---:|:---:|---|---|
| **1** | CSV/API ingestion plus tenant-owned versioned update; re-upload does not create another enrolment. | `go test -v ./internal/upload ./cmd/server` & `make smoke` (Steps d, e) | Dev A | **PASS** | `internal/upload/idempotency_test.go:TestUploadIdempotency_APIKeyReplay`, `cmd/server/server_test.go:TestPatchEnrolment_StaleVersionConflict`, `cmd/smoke/main.go` Step d/e | Unique on `(institution_id, academic_year, admission_reference)`. Version incremented on update; stale version yields 409 Conflict. Identity fields cannot be overwritten. |
| **2** | Template includes required intake, enrolment, supernumerary, NRI, OCI and FN fields; policy provenance documented. | `GET /institution/template.csv` or `cat config/policy.json` | Dev B | **PASS** | `config/policy.json`, `docs/policy.md`, `internal/validate/policy_test.go` | All 15 required CSV header columns validated. Explicit versioned policy `AICTE-2024-V1`. |
| **3** | V1-V9 demonstrable, with row/cohort/institution outcomes and review explanations. | `go test -v ./internal/validate` & `scripts/fixtures/test_admissions.csv` golden test | Dev B | **PASS** | `internal/validate/golden_test.go:TestGoldenFixtureValidation_ExactCounts`, `scripts/fixtures/golden_expected.json`, `internal/validate/rules_test.go` | Table-driven tests for V1 (syntax), V2 (mock registry), V3 (file dup), V4 (DB dup), V5 (intake overflow), V6 (policy), V7 (cross-institution lead), V8 (fuzzy matching lead), V9 (volume anomaly). |
| **4** | Institution dashboard limited to own data; AICTE aggregate dashboard refreshes after accepted processing. | Web UI: Login as `dtu.admin@institution.edu` vs `coep.admin@institution.edu`; `GET /aicte/dashboard` | Dev C | **PASS** | `cmd/server/server_test.go:TestSystematicCrossTenantIsolation_EveryEntity`, `internal/intake/intake_test.go:TestConsistency_Dashboard_RowOutcomes_Report_RollupRebuild` | Strict SQL query tenant enforcement (`institution_id = $1`). AICTE dashboard reads purely aggregated figures from `rollup_enrolment`. |
| **5** | Verifier denied without consent; permitted fields visible with consent; next request denied after revocation. | `make smoke` (Steps h, i, j, k, l) & `go test -v ./internal/credentials` | Dev D | **PASS** | `internal/credentials/trust_test.go:TestTrustChain_HappyPath_And_All8FailureModes`, `cmd/smoke/main.go` Steps h-l | Zero authorization caching. Instantaneous revocation enforcement. Only permitted fields disclosed. |
| **6** | Institution-attested signed credential; altered payload fails verification; expired/revoked credential is not current. | `go test -v ./internal/credentials` & `GET /credentials/:id/certificate` | Dev D | **PASS** | `internal/credentials/trust_test.go:TestTrustChain_HappyPath_And_All8FailureModes`, `internal/credentials/trust_test.go:TestTrustChain_ProductionSigningKeyEnforcement` | Canonical JSON payload Ed25519-signed. Tampered signature fails immediately. Expired and revoked credentials reported non-current. |
| **7** | Documented upload/status/update/verification APIs, scoped API keys and one tested signed flag webhook. | `make smoke` (Steps c, d, e), `go test -v ./cmd/server -run TestOpenAPI`, `GET /docs/api` | Dev A / Dev C / Dev D | **PASS** | `cmd/server/openapi_test.go:TestOpenAPI_RouteAndSchemaParity`, `docs/openapi.yaml`, `internal/webhook/webhook_test.go` | OpenAPI 3.0 document served offline at `/docs/api` with zero CDNs. HMAC-SHA256 signed flag webhooks with bounded retries and local demo receiver. |
| **8** | Role and tenant tests, minimal student consent, field restrictions, protected audit events and limits. | `go test -v ./cmd/server -run TestRole` & `go test -v ./internal/audit` | Dev A | **PASS** | `cmd/server/server_test.go:TestRoleAuthorizationMatrix_Comprehensive`, `internal/match/matcher_test.go:TestFuzzyMatchingSanity_ThresholdsAndBounds`, `internal/audit/chain_test.go` | Role authorization matrix enforced. Hash-chained tamper-evident audit log with SHA-256 links. Audit table revokes UPDATE/DELETE privileges. |
| **9** | CSV policy report and printable report; native PDF remains optional unless expressly required by original statement. | `GET /institution/reports` & `GET /reports/printable` | Dev B | **PASS** | `internal/intake/intake_test.go:TestConcurrentIntakeReconciliation_LargeScaleMultiInstitution`, `internal/reports/reports_test.go` | Aggregated reports for intake, categories, and flags. UTF-8 BOM + formula injection sanitization (`SanitizeCSVCell`). A4 print-ready HTML view. |
| **10** | Printable certificate containing required student/APAAR identifiers and opaque QR; public QR reveals no profile. | `GET /credentials/:id/certificate` & `GET /verify/:opaque_id` | Dev C | **PASS** | `internal/credentials/trust_test.go:TestTrustChain_NonEnumerationEnvelopeConsistency`, `docs/evidence/walkthrough_checklist.md`, `cmd/server/server_test.go:TestPublicVerification_MinimalResponse` | Clean A4 `@media print` CSS. Opaque unguessable ID. Public landing page displays only validity state, zero personal data. |

---

## 2. Systematic Role Authorization Matrix

Every route registered in the application router is mapped against all four system roles and unauthenticated callers:

| Route Path | Method | Purpose | Unauthenticated | AICTE Admin | Institution Admin | Student | Verifier |
|---|:---:|---|:---:|:---:|:---:|:---:|:---:|
| `/healthz` | GET | Health probe | 200 OK | 200 OK | 200 OK | 200 OK | 200 OK |
| `/login` | GET / POST | Authentication | 200 / 303 | Redirect Home | Redirect Home | Redirect Home | Redirect Home |
| `/logout` | POST | Invalidate session | 303 Login | 303 Login | 303 Login | 303 Login | 303 Login |
| `/verify/:opaque_id` | GET | Public QR landing | 200 (Minimal) | 200 (Minimal) | 200 (Minimal) | 200 (Minimal) | 200 (Minimal) |
| `/credentials/:id/qr.png`| GET | Credential QR image | 200 Image | 200 Image | 200 Image | 200 Image | 200 Image |
| `/credentials/:id/certificate`| GET | Printable certificate | 200 HTML | 200 HTML | 200 HTML | 200 HTML | 200 HTML |
| `/docs/openapi.yaml` | GET | OpenAPI raw spec | 200 YAML | 200 YAML | 200 YAML | 200 YAML | 200 YAML |
| `/docs/api` | GET | OpenAPI offline UI | 200 HTML | 200 HTML | 200 HTML | 200 HTML | 200 HTML |
| `/reports/printable` | GET | Printable report | 303 Redirect | 200 HTML | 200 HTML (Scoped) | 403 Forbidden | 403 Forbidden |
| `/aicte/dashboard` | GET | National rollups | 303 Redirect | **200 OK** | 403 Forbidden | 403 Forbidden | 403 Forbidden |
| `/aicte/audit/verify` | GET | Hash chain check | 303 Redirect | **200 OK** | 403 Forbidden | 403 Forbidden | 403 Forbidden |
| `/aicte/reviews` | GET | Review queue | 303 Redirect | **200 OK** | 403 Forbidden | 403 Forbidden | 403 Forbidden |
| `/aicte/reviews/:id/decision` | POST | Review resolution | 303 Redirect | **200 OK** | 403 Forbidden | 403 Forbidden | 403 Forbidden |
| `/aicte/reports` | GET | National reports | 303 Redirect | **200 OK** | 403 Forbidden | 403 Forbidden | 403 Forbidden |
| `/aicte/reports/csv` | GET | National CSV export | 303 Redirect | **200 OK** | 403 Forbidden | 403 Forbidden | 403 Forbidden |
| `/aicte/webhooks` | GET | Webhook deliveries | 303 Redirect | **200 OK** | 403 Forbidden | 403 Forbidden | 403 Forbidden |
| `/institution/dashboard`| GET | Tenant home | 303 Redirect | 403 Forbidden | **200 OK** (Scoped) | 403 Forbidden | 403 Forbidden |
| `/institution/credentials`| GET | Credential portal | 303 Redirect | 403 Forbidden | **200 OK** (Scoped) | 403 Forbidden | 403 Forbidden |
| `/institution/api-keys` | GET / POST | Key management | 303 Redirect | 403 Forbidden | **200 OK** (Scoped) | 403 Forbidden | 403 Forbidden |
| `/institution/reports` | GET | Tenant reports | 303 Redirect | 403 Forbidden | **200 OK** (Scoped) | 403 Forbidden | 403 Forbidden |
| `/institution/upload` | POST | Web CSV upload | 303 Redirect | 403 Forbidden | **200 OK** (Scoped) | 403 Forbidden | 403 Forbidden |
| `/credentials/:id/revoke`| POST | Revoke credential | 303 Redirect | 403 Forbidden | **303 OK** (Scoped) | 403 Forbidden | 403 Forbidden |
| `/student/consent` | GET | Student consent UI | 303 Redirect | 403 Forbidden | 403 Forbidden | **200 OK** (Scoped) | 403 Forbidden |
| `/consents` | POST | Grant consent | 303 Redirect | 403 Forbidden | 403 Forbidden | **303 OK** (Scoped) | 403 Forbidden |
| `/consents/:id/revoke` | POST | Revoke consent | 303 Redirect | 403 Forbidden | 403 Forbidden | **303 OK** (Scoped) | 403 Forbidden |
| `/verifier/verify` | GET | Verifier portal | 303 Redirect | 403 Forbidden | 403 Forbidden | 403 Forbidden | **200 OK** |
| `/verifier/lookup` | POST | Verifier search | 303 Redirect | 403 Forbidden | 403 Forbidden | 403 Forbidden | **200 OK** (Consent) |
| `/api/v1/uploads` | POST | Ingest admission batch | 401 Unauthorized | 401 (needs key) | **202 Accepted** (`uploads:write`) | 401 / 403 | 403 Forbidden |
| `/api/v1/uploads/:id` | GET | Poll job outcome | 401 Unauthorized | 401 (needs key) | **200 OK** (`uploads:read`, Scoped) | 401 / 403 | 403 Forbidden |
| `/api/v1/enrolments/:id`| PATCH| Patch enrolment | 401 Unauthorized | 401 (needs key) | **200 OK** (`enrolments:write`, Scoped) | 401 / 403 | 403 Forbidden |
| `/api/v1/credentials/:id/verify` | GET | Verify credential | 401 Unauthorized | 401 / 403 | 403 Forbidden | 403 Forbidden | **200 OK** (`credentials:verify` + Consent) |
| `/demo/webhook-receiver` | POST | Webhook receiver | **200 OK** (HMAC) | 200 OK (HMAC) | 200 OK (HMAC) | 200 OK (HMAC) | 200 OK (HMAC) |

---

## 3. Security Sweep Checklist

- [x] **Passwords Hashed**: Tested in `internal/auth/auth_test.go:TestPasswordHashing`. Uses standard bcrypt.
- [x] **Tokens Hashed in DB**: Sessions and API keys store high-entropy random secrets only as SHA-256 hashes (`HashToken`).
- [x] **Session Cookies Hardened**: `HttpOnly=true`, `SameSite=Lax`, `Path="/"`.
- [x] **CSRF Protection**: Enforced on all mutating browser routes (`CSRFMiddleware`). API routes authenticated via `X-API-Key` and webhooks bypass session CSRF safely.
- [x] **Parameterized SQL Only**: Zero string-concatenated SQL queries in data layers. Grep verification confirms all queries use `$1, $2, ...` placeholders.
- [x] **Upload Bounds**: Maximum 10MB memory, 5,000 rows, 256 bytes per field, strictly enforced streaming CSV parser.
- [x] **Template Auto-Escaping**: html/template sanitizes all dynamic user variables; XSS injection (`<script>alert(1)</script>`) neutralized.
- [x] **No Secrets in Repo**: Signing keys loaded from environment variable `SIGNING_PRIVATE_KEY` or generated ephemerally; API key secrets shown once and never stored raw.
- [x] **No Personal Data in Logs**: Audit logs record IDs, actions, and metadata devoid of names, DOBs, or raw APAAR IDs (Non-negotiable Rule #7).
- [x] **Audit Tamper-Evidence**: Genesis-to-head SHA-256 hash chaining with transactional advisory locks; database migration revokes `UPDATE`/`DELETE` on `audit_events`.
