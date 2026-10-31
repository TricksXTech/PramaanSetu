# Demo Path Screen Records & Visual Verification

**System**: APAAR Student Data Verification System for AICTE  
**Milestone**: Milestone 7 (Feature Freeze & Final Rehearsals)  
**Location**: `/docs/evidence/final/demo_screens.md`  
**Latest Verified Commit**: `2df1253`  

This document captures the exact visual states and data payloads of every screen traversed during the 5-minute presentation path.

---

### Screen 1: Unified Sign-In (`GET /login`)
* **Visual Components**:
  - Clean card with Email and Password inputs.
  - Pre-seeded account shortcuts card: AICTE Admin, Institution Admin (DTU & COEP), Student, Verifier.
  - CSRF hidden token in form.
* **Verified State**: Instant one-click login without typing passwords.

---

### Screen 2: Institution Dashboard & Batch Upload (`GET /institution/dashboard`)
* **Visual Components**:
  - Institution Header: `Delhi Technological University (DTU)` &bull; AICTE ID: `AICTE-DL-001`.
  - Approved Intake Quota Gauge: B.Tech CSE (Regular Approved Cap: **2**, Supernumerary: **0**).
  - Upload Box: Drag-and-drop CSV selector (`test_admissions.csv`).
  - HTMX Streaming Progress Bar: Real-time chunk progress percentage (0% $\to$ 100%).
* **Verified State**: File parsed and queued into River worker without page refresh.

---

### Screen 3: Granular Row Outcomes (`GET /jobs/:id/rows`)
* **Visual Components**:
  - Filter Tabs: `All (10)`, `Accepted (1)`, `Flagged Held (7)`, `Rejected Invalid (1)`, `Rejected Parse (1)`.
  - Detailed Table:
    - **Row 1** (`123456789012`, Rohan Gupta): `ACCEPTED` &bull; Enrolment created.
    - **Row 3** (`12345678901`): `REJECTED_INVALID` &bull; **Rule V1**: APAAR ID must be exactly 12 numeric digits.
    - **Row 4** (`999988887777`, John Doe): `FLAGGED_HELD` &bull; **Rule V6**: Foreign National status contradicts Indian nationality.
    - **Row 5-8** (Admissions 3-6): `FLAGGED_HELD` &bull; **Rule V5**: Approved regular intake capacity exceeded (Cap: 2).
    - **Row 9** (`123456789012`): `FLAGGED_HELD` &bull; **Rule V3**: Duplicate APAAR ID within same roster file.
* **Verified State**: Zero existing records overwritten; plain-language regulatory reasons displayed.

---

### Screen 4: AICTE National Review Queue (`GET /aicte/reviews`)
* **Visual Components**:
  - **Cross-Institution Concurrent Study (Rule V7)**:
    - Masked Identifier: `XXXX-XXXX-1234` (Priya Sharma).
    - Evidence: Concurrently enrolled in B.Tech CSE at DTU (Delhi) and COEP (Pune) for Academic Year `2024-25`.
  - **Fuzzy Identity Candidate Lead (Rule V8)**:
    - Match Score: `0.73` ($\ge 0.65$ threshold).
    - Candidates: `"Aarav Sharma"` (COEP) vs `"Arav Sharma"` (DTU), identical DOB `2004-05-15` and father `"Rajesh Sharma"`.
  - Resolution Actions: Dropdown (`Confirmed Different People`, `Confirmed Same Person`, `Valid Concurrent Study`) + mandatory audited reviewer note.
* **Verified State**: Privacy protected (zero raw APAAR IDs or unmasked personal data in list view); resolution appends to audit log.

---

### Screen 5: AICTE National Dashboard & Rollup Aggregation (`GET /aicte/dashboard`)
* **Visual Components**:
  - National Aggregate Cards: Total Admissions, Total Active Institutions, Open Leads.
  - State-wise Admission Breakdown table.
  - Institution Capacity Adherence Table: Displays approved limits vs accepted admissions.
  - Volume Anomaly Detector (**Rule V9**): Multi-year historical baseline comparison. Spikes > 2 standard deviations flagged with visual warning badge.
* **Verified State**: Real-time aggregated data queried from `rollup_enrolment` with zero polling lag.

---

### Screen 6: Institution Attestation & Credential Issuance (`GET /institution/credentials`)
* **Visual Components**:
  - Attested Enrolments Table: Student Name, Admission Reference, APAAR ID, Program.
  - Action Button: **"Attest & Issue Verifiable Credential"**.
  - Generated Output:
    - 64-byte Ed25519 digital signature.
    - High-entropy unguessable Opaque ID (`opq_...`).
    - Cryptographic QR code.
* **Verified State**: Enrolment version frozen at attestation (v1); tampering or version increment invalidates signature.

---

### Screen 7: Verifier Portal & Dynamic Consent Gate (`GET /verifier/verify`)
* **State A: Verification WITHOUT Active Student Consent**:
  - Input: Credential UUID or Opaque ID.
  - Verification Indicator: `Format Valid: YES`, `Registry Matched: YES`, `Institution Attested: YES`, `Signature Valid: YES`.
  - **Access Gate**: Status displays **`CONSENT REQUIRED`**.
  - **Personal Data**: **NONE** disclosed (`Permitted Fields: nil`). Zero personal leakage!
* **State B: Verification WITH Active Student Consent**:
  - Student approves request from TCS Campus HR (`Purpose: Background Check`, `Permitted: Name, Program, Status`).
  - Verifier re-queries: Discloses **strictly permitted fields** (*Name: Rohan Gupta, Program: B.Tech CSE, Status: Attested*).
* **State C: Immediate Revocation Enforcement**:
  - Student clicks "Revoke Consent". Verifier re-queries immediately: **`403 Forbidden`** (Zero authorization caching).

---

### Screen 8: Printable Credential Certificate (`GET /credentials/:id/certificate`)
* **Visual Components**:
  - Double border with official AICTE header.
  - Attested Student Name, Institutional ID, National APAAR ID, Program, Admission Year.
  - Live Verification Status Badge (`Active & Valid`).
  - Crisp Opaque QR Code linking to `/verify/:opaque_id`.
  - Dedicated `@media print` styling: Navigation bar and action buttons automatically vanish on `Ctrl+P`; fits on single A4 portrait page.

---

### Screen 9: Public Opaque QR Landing Page (`GET /verify/:opaque_id`)
* **Visual Components**:
  - Publicly accessible without authentication.
  - Displays **only** status: *"Valid Credential - Issued by Delhi Technological University"*.
  - **Zero Personal Data**: Completely devoid of student name, photo, DOB, guardian, or APAAR ID.

---

### Screen 10: Genesis-to-Head Audit Verification (`GET /aicte/audit/verify`)
* **Visual Components**:
  - Hash chain status card: Total events verified, genesis hash, head hash.
  - Visual verification check: **"0 Broken Links - Audit Chain Intact"**.
  - Simulated Tamper Test: Flips a single byte in an audit record; system immediately flags exact broken link number.

