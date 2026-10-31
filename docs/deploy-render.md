# Render Cloud Deployment Guide (with PostgreSQL 16)

**Project**: PramaanSetu (प्रमाणसेतु) &bull; APAAR Student Data Verification System for AICTE  
**Platform**: Render Cloud ([dashboard.render.com](https://dashboard.render.com/))  
**Database**: Managed PostgreSQL 16 (`pg_trgm`, `fuzzystrmatch` extensions enabled)  
**Repository**: `https://github.com/TricksXTech/PramaanSetu`  

---

## Method 1: One-Click Blueprint Deployment (Recommended)

Render provides an Infrastructure-as-Code Blueprint engine using our root [`render.yaml`](file:///d:/SIH/render.yaml).

### Steps:
1. Open your browser and navigate to the Render Blueprint Dashboard:  
   👉 **[https://dashboard.render.com/blueprints](https://dashboard.render.com/blueprints)**
2. Click **"New Blueprint Instance"** (or top-right **"New +" &rarr; "Blueprint"**).
3. Connect your GitHub account and select the repository:  
   **`TricksXTech/PramaanSetu`**
4. Render automatically parses [`render.yaml`](file:///d:/SIH/render.yaml) and prepares:
   * **Database**: `pramaansetu-db` (PostgreSQL 16, free tier, database `apaar_verification`).
   * **Web Service**: `pramaansetu` (Docker runtime, auto-wired `DATABASE_URL`, health check `/healthz`).
5. Click **"Apply"**.
6. Render builds the Docker image, applies database migrations, auto-seeds default AICTE/Institution accounts, and assigns a live public URL (e.g. `https://pramaansetu.onrender.com`).

---

## Method 2: Manual Dashboard Setup (Step-by-Step)

If you prefer configuring resources individually in the Render Dashboard:

### Step 1: Create PostgreSQL Database on Render
1. Go to **[https://dashboard.render.com/new/database](https://dashboard.render.com/new/database)**.
2. Configure the database parameters:
   * **Name**: `pramaansetu-db`
   * **Database**: `apaar_verification`
   * **User**: `apaar_user`
   * **Region**: `Oregon (US West)` (or closest region)
   * **PostgreSQL Version**: `16`
   * **Instance Type**: `Free`
3. Click **"Create Database"**.
4. Once created, scroll down to the **"Connections"** section and copy the **Internal Database URL** (e.g. `postgres://apaar_user:password@dpg-xxx-a:5432/apaar_verification`).

---

### Step 2: Create Web Service on Render
1. Go to **[https://dashboard.render.com/select-repo?type=web](https://dashboard.render.com/select-repo?type=web)**.
2. Select repository: **`TricksXTech/PramaanSetu`** (Branch: `main`).
3. Fill in the service configuration:
   * **Name**: `pramaansetu`
   * **Region**: Same region as database (e.g. `Oregon`)
   * **Language / Runtime**: **`Docker`**
   * **Dockerfile Path**: `./Dockerfile`
   * **Instance Type**: `Free`
4. Expand **"Advanced"** and add the following **Environment Variables**:

| Key | Value | Description |
|---|---|---|
| `DATABASE_URL` | *(Paste Internal Database URL from Step 1)* | Full connection string to Render PostgreSQL |
| `PORT` | `10000` | Port required by Render web routing |
| `AUTO_SEED` | `true` | Automatically seeds demo accounts and quotas on boot |
| `SESSION_SECRET` | `pramaansetu-render-session-secret-key-32` | 32-character cookie encryption secret |
| `ENVIRONMENT` | `production` | Production runtime flag |

5. Set **Health Check Path**: `/healthz`
6. Click **"Create Web Service"**.

---

## Step 3: Live Verification & Testing

Once Render finishes the initial build (approx. 2 minutes):

1. **Verify Health**:
   Open `https://<your-app-name>.onrender.com/healthz`  
   Expected JSON response:
   ```json
   {
     "database": "healthy",
     "status": "healthy",
     "timestamp": "2026-10-31T11:35:00Z",
     "version": "0.3.0-milestone3"
   }
   ```

2. **Access Web Application**:
   Navigate to `https://<your-app-name>.onrender.com/login`

3. **Sign In with Pre-Seeded Accounts** (Password: `Password@123`):
   * **AICTE National Admin**: `admin@aicte.gov.in`
   * **Institution Admin (DTU)**: `admin@dtu.ac.in`
   * **Institution Admin (COEP)**: `admin@coep.ac.in`
   * **Student Account**: `student1@example.com`
   * **Employer / Verifier**: `verifier@tcs.com`

---

## Troubleshooting Common Cloud Quirks

* **Database Connection Timeout on Boot**:  
  `cmd/server/main.go` includes an automatic 10-attempt retry loop with 2-second backoff. If Render's database takes 10–15 seconds to finish provisioning on first run, the app waits automatically.
* **Database Extensions (`pg_trgm`, `fuzzystrmatch`)**:  
  Render's PostgreSQL 16 grants extension creation permissions to `apaar_user`. The server runs `migrations/001_initial_schema.sql` automatically on startup to install them.
* **Auto-Sleep on Free Tier**:  
  Free web services on Render spin down after 15 minutes of inactivity. When a new request arrives, it spins up in ~30 seconds; the health check and auto-seed ensure instant recovery without data loss.
