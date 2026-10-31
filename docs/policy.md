# AICTE Regulatory Policy & Intake Reconciliation Specifications

## 1. Documented Hold Policy (Rule V5)
When an institution submits student admission records for an approved programme and academic year:
1. **Intake Cap & Serialization**:
   - The regular seat cap is specified by `approved_intake.regular_limit`.
   - The admission reconciliation transaction strictly serializes per `(institution_id, program_id, academic_year)` using a pessimistic row lock (`SELECT ... FOR UPDATE` on `approved_intake`).
2. **First-Come / Row-Order Acceptance**:
   - Candidates are evaluated sequentially in the exact submitted row order.
   - Rows arriving up to the configured limit are **accepted** and promoted to `enrolments`.
   - Any regular candidates arriving after the cap has been exhausted are marked as **`flagged_held`** with rule code `V5: Intake Overflow`.
   - Held rows are **never silently dropped or discarded**; they are fully preserved in `upload_rows` and flagged for institutional/AICTE administrative review.
3. **Supernumerary Quotas**:
   - Supernumerary, NRI, OCI, and Foreign National (FN) seats do not consume regular seats.
   - They are bounded separately against their respective statutory allowances (e.g. 5% NRI, 5% FN, 5% Tuition Fee Waiver).
   - If a supernumerary category exceeds its statutory allowance, subsequent rows are marked `flagged_held`.

## 2. Policy Consistency Rules (Rule V6)
Policy validation is enforced dynamically via `PolicyConfig` (current active version: `AICTE-2024-V1`):
- **Age Window**: Student age at admission date/year must be within 16 to 40 years.
- **Nationality & FN Consistency**: Candidates tagged as `is_foreign_national = true` must provide a non-Indian foreign nationality. Tagging FN with nationality `INDIAN` triggers a V6 conflict flag.
- **Mutual Exclusivity**: A candidate cannot simultaneously be marked as both `is_nri = true` and `is_foreign_national = true`.
- **OCI Status**: OCI (Overseas Citizen of India) status requires an associated foreign passport/nationality; Indian nationality cannot be set for OCI holders.
- **Supernumerary Validation**: Candidates flagged `is_supernumerary = true` must have an authorized admission type or allowance category.

