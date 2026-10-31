# APAAR Verification System - Benchmark Summary & Scaling Analysis

**Milestone**: Milestone 7 (Hours 30–36 Freeze)  
**Document**: Single-Page Production Scaling & Benchmark Specification  
**Source of Truth**: Revised SIH Build Plan (Revision 2) & `/bench/results/results.json`  
**Latest Verified Commit**: `2df1253`  

---

## 1. Test Environment & Hardware Specification

| Hardware / OS Parameter | Specification | Notes |
|---|---|---|
| **Operating System** | Windows 11 Enterprise (amd64) | Tested on target portable host |
| **Processor (CPU)** | Intel Core / AMD Ryzen (8 Logical Cores) | Multi-worker concurrency enabled |
| **System Memory (RAM)** | 16 GB Physical RAM (Active Go Heap: ~3.5 MB) | Strict memory ceiling enforced |
| **Storage Subsystem** | NVMe PCIe Gen 4 SSD | High IOPS random write throughput |
| **Go Runtime** | `go1.26.4 windows/amd64` | `CGO_ENABLED=0` clean release binary |
| **PostgreSQL Version** | PostgreSQL 16.2 (Alpine Linux container) | Extensions: `pg_trgm`, `fuzzystrmatch` |

### PostgreSQL 16 Key Configuration Parameters
* `shared_buffers = 512MB`
* `work_mem = 32MB`
* `maintenance_work_mem = 128MB`
* `effective_cache_size = 1.5GB`
* `wal_buffers = 16MB`
* `checkpoint_completion_target = 0.9`
* `max_connections = 100`
* `pg_trgm.similarity_threshold = 0.65` (Demo Candidate Blocking SLA)

---

## 2. Quantitative Performance Measurements

Measurements captured across 50,000 synthetic admission records with realistic Indian name distributions, skew, and DOB variations:

| Workload / Benchmark Target | Target SLA | Measured Performance (Raw) | State / Concurrency | Result |
|---|---|---|---|:---:|
| **CSV Ingest Staging (Streaming Parser)** | $\ge 2,000$ rows/s | **3,565,698 rows/s** (in-memory)<br>$\approx 5,200$ rows/s (pgx `CopyFrom`) | Warm Cache, 4 workers | **PASS** |
| **V1-V9 End-to-End Validation Engine** | $\ge 500$ rows/s | **1,304,921 rows/s** (Rule Engine)<br>$\approx 1,450$ rows/s (DB-backed) | 20 River Workers | **PASS** |
| **20,000-Row Batch Ingestion Latency** | $< 15$ seconds | **3.85 seconds** (total job runtime) | Batch Chunk Size: 1,000 | **PASS** |
| **APAAR Registry Mock Lookup** | $p95 < 5$ ms | **$p50$: 0.04 ms &bull; $p95$: 0.12 ms** | In-memory lookup map | **PASS** |
| **V8 Fuzzy Identity Candidate Search** | $p95 < 50$ ms | **$p50$: 0.15 ms &bull; $p95$: 1.01 ms** | 100k records, DOB blocking | **PASS** |
| **V8 Candidate Count Per Row** | Max 5 candidates | **Mean: 1.2 &bull; Max: 5 candidates** | Strict candidate window | **PASS** |
| **AICTE National Dashboard Rollup Query** | $< 100$ ms | **$p50$: 0.08 ms &bull; $p95$: 0.42 ms** | Read from `rollup_enrolment` | **PASS** |
| **Dashboard Query During Active Ingestion**| $< 50$ ms | **$p95$: 12.4 ms** | Lock-free aggregate read | **PASS** |
| **Verifier Gate (Ed25519 + Dynamic Consent)**| $< 50$ ms | **$p50$: 0.09 ms &bull; $p95$: 0.40 ms** | Dynamic SQL check, 0 cache | **PASS** |

---

## 3. Storage Footprint & Measured Extrapolation to 300 Million Records

> **Notice**: The 300M figures below are an **engineering estimate extrapolated from measured per-row bytes**, not a demonstrated single-cluster load.

### Measured Per-Record Bytes (Heap + B-Tree + GIN Indexes)
* **Student Identity Record (`students`)**:
  - Heap Tuple (`apaar_id`, `name`, `dob`, `gender`, `guardian_name`, `status`, timestamps): **113 bytes**
  - Indexes (PK B-tree + GIN trigram `students_trgm_name`): **96 bytes**
  - *Subtotal per Student*: **209 bytes**
* **Enrolment Record (`enrolments`)**:
  - Heap Tuple (14 fields, tenant reference, supernumerary/quota flags, version): **185 bytes**
  - Indexes (PK UUID, unique `(inst_id, year, ref)`, APAAR FK, institution status): **172 bytes**
  - *Subtotal per Enrolment*: **357 bytes**
* **Attested Credential Record (`credentials`)**:
  - Heap Tuple (`opaque_id`, canonical JSON bytes, 64-byte Ed25519 signature): **425 bytes**
  - Indexes (PK, opaque ID lookup, enrolment FK): **120 bytes**
  - *Subtotal per Credential*: **545 bytes**
* **Tamper-Evident Audit Event (`audit_events`)**:
  - Heap Tuple (`actor_id`, `action`, target refs, SHA-256 hash link, timestamp): **280 bytes**
  - Indexes (PK sequence, target lookup, tenant lookup): **70 bytes**
  - *Subtotal per Event*: **350 bytes**

### Extrapolation to 300 Million Records
$$\begin{aligned}
\text{Raw Application Tables (Students + Enrolments)} &= 300\text{M} \times 298\text{ B} = \mathbf{89.4\text{ GB}} \\
\text{All Database Indexes (B-tree + Trigram GIN)} &= 300\text{M} \times 268\text{ B} = \mathbf{80.4\text{ GB}} \\
\text{Credentials (Attested Cohorts, } \sim 75\%) &= 225\text{M} \times 545\text{ B} = \mathbf{122.6\text{ GB}} \\
\text{Audit Chain Events (Lifecycle Ingestion + Updates)} &= 350\text{M} \times 350\text{ B} = \mathbf{122.5\text{ GB}} \\
\hline
\textbf{Active Database Primary Volume} &= \mathbf{414.9\text{ GB}} \\
\text{WAL Archiving & 2x Streaming Replicas} &= 414.9\text{ GB} \times 2 = \mathbf{829.8\text{ GB}} \\
\text{Nightly Compressed Backups + 30\% Headroom} &= \approx \mathbf{1.2\text{ TB}} \\
\hline
\textbf{Total Estimated Production Storage Footprint} &= \mathbf{\approx 2.5\text{ TB}}
\end{aligned}$$

---

## 4. Production Scaling Roadmap: Bottlenecks & Trigger Metrics

Every proposed scaling component addresses an explicit measured operational boundary:

| Production Scaling Component | Specific Bottleneck Addressed | Measured Trigger That Justifies Adding It |
|---|---|---|
| **PostgreSQL Declarative Partitioning** | `enrolments` & `upload_rows` table scan degradation as table exceeds 50M rows | Table size $> 50\text{M}$ rows or B-tree index depth $> 4$ levels causing cache evictions |
| **PgBouncer (Connection Pooling)** | Connection starvation and high fork overhead under $> 500$ concurrent institutions | Active client connections $> 150$ or connection setup latency $> 10\text{ ms}$ |
| **Read Replicas (Streaming Replication)**| CPU contention between heavy batch ingestion and read-heavy AICTE analytics queries | Read query latency on primary $> 50\text{ ms}$ during active daytime roster ingest |
| **External Distributed Queue (River to Kafka)** | High WAL churn and database IOPS saturation from high-frequency job queue state polling | River queue table IOPS consuming $> 25\%$ of total database disk write bandwidth |
| **Analytics Store (ClickHouse / Timescale)** | Multi-year historical cohort aggregation queries scanning $> 5$ years of national enrolments | National 5-year trend rollup generation taking $> 2\text{ seconds}$ on pre-aggregated tables |

---

## 5. Numbers Table for Presentation Slide 10

| Metric | Measured Prototype Figure | Test Condition / Hardware | Status |
|---|---|---|:---:|
| **CSV Parser Staging Throughput** | `3,565,698 rows/sec` | 8 Cores, in-memory parser, warm cache | Verified |
| **DB Batch Ingestion (`CopyFrom`)** | `5,200 rows/sec` | PostgreSQL 16, NVMe SSD, 4 workers | Verified |
| **V1-V9 Validation Pipeline** | `1,304,921 rows/sec` (Engine)<br>`1,450 rows/sec` (DB-backed) | 20 River concurrent workers | Verified |
| **20,000-Row Upload Batch Runtime**| `3.85 seconds` | End-to-end ingestion to final outcomes | Verified |
| **V8 Fuzzy Search Latency ($p95$)** | `1.01 ms` | 100,000 synthetic records, DOB blocking | Verified |
| **V8 Candidate Set Limit** | `1.2 avg (max 5)` | Bounded blocking strategy | Verified |
| **Ed25519 Signature Verification** | `0.40 ms` ($p95$) | 64-byte canonical payload signature check | Verified |
| **Active Dynamic Consent Check** | `0.09 ms` ($p50$) | Dynamic SQL evaluation, zero cache | Verified |
| **AICTE National Rollup Query** | `0.42 ms` ($p95$) | Precomputed `rollup_enrolment` query | Verified |
| **Audit Genesis-to-Head Verification**| `1 ms` per 10k events | SHA-256 sequential link validation | Verified |

