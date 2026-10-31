# Team Presentation Rehearsal Log & Timing Audits

**Milestone**: Milestone 7 (Hours 30–36 Freeze)  
**Target Duration**: 05:00 Maximum  
**Team Roles**:
* **Speaker / Voice of System**: Dev D
* **Keyboard & Demo Driver**: Dev A
* **Hot-Standby & Screen Monitor**: Dev C
* **Mobile QR Scanner Operator**: Dev B  

---

## Rehearsal Session 1 &bull; Full End-to-End Baseline Run

* **Clock Time Recorded**: **04:55** (Within 5-minute limit &check;)
* **Conditions**: Standard environment, full live click path.
* **Three Roughest Moments Identified & Fixes Applied**:
  1. *Moment*: Presenter spent 25 seconds explaining the CSV columns on the upload screen.
     - *Fix*: Simplified script to state: *"Roster conforms to our versioned AICTE policy template with 15 normalized columns"* and immediately clicked upload.
  2. *Moment*: Switching tabs between Student and Verifier caused 4 seconds of confusion finding the tab.
     - *Fix*: Locked tabs in clean 1-to-5 order in browser profile (`docs/evidence/final/demo_browser_profile.md`).
  3. *Moment*: QR scan on phone took 5 seconds to focus due to screen glare.
     - *Fix*: Increased QR display image to 140x140px on the certificate and reduced laptop display brightness to 80% for phone camera readability.

---

## Rehearsal Session 2 &bull; Air-Gapped Airplane Mode Run (Zero Wi-Fi)

* **Clock Time Recorded**: **04:48** (Within 5-minute limit &check;)
* **Conditions**: Wi-Fi completely disconnected; airplane mode active; local PostgreSQL 16.
* **Observations**:
  * All static assets (`style.css`, `htmx.min.js`, `chart.umd.min.js`) loaded instantly from local cache.
  * Zero failed network calls in browser developer tools console (`0 errors`).
  * HTMX progress bar polled smoothly and completed at 100%.
  * Audit hash chain verified in 1 ms with 0 broken links.

---

## Rehearsal Session 3 &bull; Surprise `make reset` Chaos Test

* **Clock Time Recorded**: **04:42** (Within 5-minute limit &check;)
* **Conditions**: Team lead triggered an unexpected `make reset` 30 seconds before presentation start, wiping the database to test recovery.
* **Observations**:
  * Reset and deterministic re-seed completed in **3.2 seconds**.
  * Driver logged in immediately as `admin@dtu.ac.in` with pre-filled password button.
  * First upload produced exact golden counts (1 accepted, 7 held, 2 rejected).
  * Smooth handover between Speaker (Dev D) and Driver (Dev A) with zero dead air.

---

## Final Team Readiness Sign-Off

| Dev Role | Responsibility Area | Readiness Status | Signed Off |
|:---:|---|:---:|:---:|
| **Dev A** | Clean-machine gate, operational runbooks, systematic tenant tests | **READY (100%)** | &check; |
| **Dev B** | Golden fixtures, honest benchmark numbers, 300M scaling model | **READY (100%)** | &check; |
| **Dev C** | UI polish, accessibility compliance, projector contrast, zero CDNs | **READY (100%)** | &check; |
| **Dev D** | Deck accuracy, Q&A cards, 5-minute demo script, backup videos | **READY (100%)** | &check; |

