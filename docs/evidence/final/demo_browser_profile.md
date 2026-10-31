# Demo Browser Profile & Stage Display Guidelines

**Owner**: Dev C  
**Target Environment**: Chrome / Chromium 120+ (Air-gapped Demo Profile)  
**Hardware Profile**: Venue Projector (1280x720 or 1920x1080)  

---

## 1. Dedicated Demo Browser Profile Setup

To avoid unexpected browser popups, extensions, auto-fill prompts, or search history suggestions during live jury evaluation:

1. **Launch Clean Profile**:
   ```bash
   google-chrome --user-data-dir=/tmp/sih_demo_profile --no-first-run --no-default-browser-check
   ```
2. **Display Settings**:
   * **Zoom Level**: **110%** (guarantees crisp readability from 15 feet away on a venue projector).
   * **Window Mode**: Maximized (F11 Fullscreen optional; standard window with bookmarks bar visible preferred).
   * **Bookmarks Bar**: Enabled (`Ctrl+Shift+B` / `Cmd+Shift+B`).
   * **Extensions**: Completely disabled.
   * **OS Notifications**: Set Windows / macOS to "Do Not Disturb" / "Focus Assist".

---

## 2. Pinned Bookmark Tabs (Left-to-Right Sequence)

The browser profile should have 5 bookmarks pinned in exact presentation order:

| Tab # | Bookmark Title | Target URL | Active Persona |
|:---:|---|---|---|
| **Tab 1** | `1. Sign In` | `http://localhost:8080/login` | Switchboard for quick sign-in |
| **Tab 2** | `2. DTU Portal` | `http://localhost:8080/institution/dashboard` | Institution Admin (`admin@dtu.ac.in`) |
| **Tab 3** | `3. AICTE National`| `http://localhost:8080/aicte/dashboard` | National Admin (`admin@aicte.gov.in`) |
| **Tab 4** | `4. Student Vault`| `http://localhost:8080/student/consent` | Student (`student1@example.com`) |
| **Tab 5** | `5. Verifier HR` | `http://localhost:8080/verifier/verify` | Background Verifier (`verifier@tcs.com`) |

---

## 3. Pre-Saved Credentials in Browser

Pre-seed the following passwords in the demo profile to eliminate manual keyboard input:
* `admin@aicte.gov.in` : `Password@123`
* `admin@dtu.ac.in` : `Password@123`
* `admin@coep.ac.in` : `Password@123`
* `student1@example.com` : `Password@123`
* `student2@example.com` : `Password@123`
* `verifier@tcs.com` : `Password@123`

*(Note: The `/login` page already provides one-click pre-fill credential buttons for all roles as a failsafe).*
