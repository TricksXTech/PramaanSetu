# Ed25519 Key Rotation Procedure & Lifecycle Specification

## Overview
All verifiable academic credentials issued by institutions in the APAAR Student Data Verification System are digitally signed using Ed25519. In compliance with **Non-negotiable Rule #8**, private signing keys are loaded from external configuration (e.g. environment variables or secure key vaults) and **never committed to the repository**.

To maintain long-term trust without invalidating existing issued credentials, the verification system supports multi-key verification while maintaining a single authoritative signing key.

---

## 1. Multi-Key Architecture
* **Primary Signing Key (`KEY_ID: ed25519_2024_01`)**: Active private key used to sign new canonical credential payloads.
* **Verification Key Ring**: A list of trusted public keys indexed by `key_id`. During verification (`VerifyCredential`), the credential's `key_id` selects the matching public key from the keyring.

```
Credential Payload + Key ID ("ed25519_2024_01")
             │
             ▼
    [Ed25519 Signer]
             │
             ▼
  Digital Signature (64 bytes)
```

---

## 2. Standard Key Rotation Procedure

### Phase 1: Key Generation & Staging (Day 0)
1. Generate a new high-entropy Ed25519 key pair with identifier `ed25519_2025_01`.
2. Deploy the application with the new public key added to the trusted verification keyring.
3. At this phase, `ed25519_2024_01` remains the active signing key.

### Phase 2: Active Key Cutover (Day 7)
1. Switch the primary signing key environment variable:
   ```bash
   CREDENTIAL_SIGNING_KEY_ID="ed25519_2025_01"
   CREDENTIAL_SIGNING_PRIVATE_KEY="<base64-encoded-private-key>"
   ```
2. New credentials issued will carry `key_id = "ed25519_2025_01"`.
3. Pre-existing credentials bearing `key_id = "ed25519_2024_01"` continue to verify cleanly against the trusted verification keyring.

### Phase 3: Retirement & Sunset (Day 365)
1. Once all credentials signed with `ed25519_2024_01` have expired or reached natural supersession, the old public key is marked `retired` in the audit log.

---

## 3. Credential Lifecycle States
Under Non-negotiable Rule #5, credential states are kept distinct:
1. **Format Valid**: Syntax conforms to schema.
2. **Registry Matched**: Verified against APAAR registry mock.
3. **Rules Passed**: V1-V9 checks satisfied.
4. **Institution Attested**: Enrolment status is `attested` and matches version.
5. **Signature Valid**: Ed25519 signature cryptographically verified under selected `key_id`.
6. **Credential Current**:
   - `revoked_at IS NULL` (not revoked).
   - `expires_at > CURRENT_TIMESTAMP` (not expired).
   - `supersedes_id` check: no newer credential issued for this enrolment.

