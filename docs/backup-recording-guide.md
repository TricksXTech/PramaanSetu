# Backup Video Recording Protocol & Offline Demonstration Archive

**Document Version**: 1.0 (Milestone 7 Release Gate)  
**Target Video Format**: MP4 (H.264 / AAC), 1080p @ 60 FPS, File Size $\le 150$ MB  
**Storage Locations**: Demo Laptop (`~/Desktop/SIH_Backups/`) & USB Root (`/video/`)  
**Single Source of Truth**: Revised SIH Build Plan (Revision 2) & `docs/demo-script.md`  

---

## 1. Recording Specifications & Setup

1. **Screen Resolution**: 1920x1080 at 100% OS scaling (Browser zoom: 110%).
2. **Audio Track**: Clean lavalier microphone narration following the exact verbal cues from `docs/demo-script.md`.
3. **Recording Tools**: OBS Studio or Windows Game Bar (`Win+G`) with clean screen capture (taskbar hidden optional; cursor highlighting enabled).
4. **Clean Baseline**: Run `make reset` immediately before recording to ensure an untampered, repeatable state.

---

## 2. Master Backup Video Track List

### Video 1: Master 5-Minute End-to-End Walkthrough (`apaar_full_demo_backup.mp4`)
* **Duration**: Exactly 04:52
* **Sequence**:
  - `00:00 - 00:58`: Login as DTU Admin &bull; Intake Cap of 2 &bull; Upload 10-row CSV &bull; Detailed Row Outcomes (V1, V3, V5, V6).
  - `00:58 - 01:42`: Login as AICTE Admin &bull; Review Queue (V7 Concurrent Study & V8 Fuzzy Match) &bull; Record Audited Resolution.
  - `01:42 - 02:28`: AICTE National Dashboard &bull; Aggregate Cards &bull; Quota Adherence &bull; Volume Anomaly Warning (V9).
  - `02:28 - 03:55`: DTU Credential Attestation &bull; Ed25519 Signing &bull; Verifier Query (Consent Required) &bull; Student Consent Grant &bull; Verifier Data Reveal &bull; Immediate Revocation.
  - `03:55 - 04:25`: Printable A4 Certificate (`Ctrl+P`) &bull; Opaque Public QR &bull; HMAC-SHA256 Webhook &bull; Audit Chain Verification (0 broken links).
  - `04:25 - 04:52`: Benchmark Summary Slide &bull; Fixed Stack Integrity &bull; Conclusion.

### Video 2: Cryptographic Tamper Defense Clip (`failure_mode_tamper.mp4`)
* **Duration**: 00:45
* **Action**:
  - Displays issued credential canonical payload bytes.
  - Modifies a single character in the payload hex.
  - Invokes verification endpoint: Instantly returns `ErrInvalidSignature: signature verification failed: invalid signature or tampered payload`.
  - Proves cryptographic authenticity defense.

### Video 3: Zero-Cache Dynamic Consent Revocation Clip (`failure_mode_revocation.mp4`)
* **Duration**: 00:50
* **Action**:
  - Verifier views student's verified name and program.
  - Student clicks "Revoke Consent" in their portal tab.
  - Verifier refreshes query: Instantly blocked with `403 Forbidden` (`consent_required: true`, `PermittedFields: nil`).
  - Proves absolute lack of cached authorization decisions.

### Video 4: Webhook HMAC-SHA256 Integrity Clip (`failure_mode_webhook.mp4`)
* **Duration**: 00:40
* **Action**:
  - Ingests batch triggering Rule V5 intake overflow flag.
  - Outbound webhook delivers signed payload with `X-Signature-SHA256`.
  - Receiver validates HMAC using shared secret.
  - Tampering with event ID or timestamp causes receiver to reject with `401 Unauthorized: Invalid HMAC signature`.

---

## 3. Playback Verification & USB Deployment

Verify that each recorded MP4 file:
1. Plays natively in VLC, QuickTime, and Windows Media Player without third-party codec prompts.
2. Contains clear, audible English audio narration without background hiss.
3. Is copied onto both physical USB drives (`Primary` and `Secondary`).

