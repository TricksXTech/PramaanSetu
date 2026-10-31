# Known Limitations & Demo Scope Boundaries

**APAAR Student Data Verification System (36-Hour Hackathon Prototype)**  
Document Version: **1.0 (Milestone 6 Freeze)**  
Single Source of Truth: **Revised SIH Build Plan (Revision 2)**

In accordance with SIH build guidelines and honest demonstration standards, this document transparently enumerates all intentional scope boundaries, mock adapters, and prototype design trade-offs.

---

### 1. Synthetic Registry Adapter
- **Implementation**: The national APAAR registry lookup is implemented via `validate.MockRegistryAdapter` ([`internal/validate/registry.go`](file:///d:/SIH/internal/validate/registry.go)).
- **Why**: Real APAAR / DigiLocker APIs require official government MoE/AICTE credentials and production whitelisting not available in a hackathon sandbox.
- **Labels & Behavior**: All verification outputs carry an explicit `provenance: "mock"` tag. Identifiers ending in `000000000404` simulate "not found", `000000000503` simulate "registry temporarily unavailable", and `000000000999` simulate "identity mismatch".

### 2. Synthetic Test Datasets
- **Implementation**: All student names, APAAR numbers, admission references, and guardian details are generated synthetic fixtures (`scripts/fixtures/test_admissions.csv`).
- **Honest Claim**: No real student records or personal data are stored, ingested, or processed.

### 3. Client-Side Print-to-PDF (No Headless Chrome)
- **Implementation**: Printable credential certificates (`/credentials/:id/certificate`) and regulatory reports (`/reports/printable`) utilize standards-compliant `@media print` A4 CSS.
- **Why**: Bundling a headless Chrome/Chromium browser or external Python/Node rendering service into the Go Docker container violates the strict two-container limit (Go App + PostgreSQL 16 only).
- **Operation**: Users export certificates and reports directly to PDF via the standard browser print dialogue (`Ctrl+P`).

### 4. Demo Regulatory Policy Values
- **Implementation**: Policy rules V5 and V6 read parameters from [`config/policy.json`](file:///d:/SIH/config/policy.json).
- **Parameters**: Values such as approved program capacity (60 seats), supernumerary allowance (10%), NRI quota (5%), and age bounds (16 to 40 years) represent demo values for testing regulatory enforcement. In production, these are configured per institution approval letters.

### 5. Single-Node Architecture & Embedded River Queue
- **Implementation**: Background validation batches and retry queues run within the single Go application binary using PostgreSQL-backed River workers.
- **Why**: The build plan strictly forbids Redis, Kafka, NATS, or Kubernetes.
- **Capacity**: Demonstrates measured throughput of $\ge 3,000$ rows/sec on staging and $\ge 1,000$ rows/sec through the validation pipeline, targeting a 300M record production roadmap without making false multi-node cluster claims.

### 6. Local Demo Webhook Receiver
- **Implementation**: Outbound HMAC-SHA256 signed flag webhooks are delivered to a local fixed demo receiver at `/demo/webhook-receiver`.
- **Deduplication & Window**: The receiver enforces a 5-minute timestamp tolerance window and deduplicates by `X-Event-Id`. Arbitrary third-party webhook URLs are restricted in the prototype.

### 7. Key Rotation Keyring
- **Implementation**: Ed25519 signing key rotation is handled via an in-memory verification keyring (`publicKeys map[string]ed25519.PublicKey`) and documented in [`docs/key-rotation.md`](file:///d:/SIH/docs/key-rotation.md). Older credentials verify cleanly after rotation without requiring immediate batch re-signing.
