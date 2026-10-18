# BYOT — Bring Your Own Tests

> Monitor your production business flows, not just your code.

BYOT is an open-source production monitoring platform. It runs the tests you already have (Playwright, k6, Locust, Node, Python, Go binaries) on a schedule inside isolated Docker containers, and alerts you when something actually breaks.

> **Scope:** BYOT is built for a single team running it on their own infrastructure. There is no multi-tenancy yet: every signed-in user can see and manage every test.

## Demo

[![Watch the 30-second launch video](docs/og.png)](docs/launch.mp4)

▶️ **[Watch the launch video](docs/launch.mp4)** (30s)

---

## Features

- **Any test runtime:** `node`, `python`, `go`, `k6`, `locust`, `playwright` (zip bundle)
- **Isolated execution:** every run gets a fresh, hardened container (`--cap-drop=ALL`, `no-new-privileges`, `--read-only`, tmpfs `/tmp`, non-root user)
- **Cron scheduling:** per-test schedules, plus a global pause/resume switch
- **Alerts:** Slack-compatible webhooks with a consecutive-failure threshold and recovery notices, so flaky tests don't page anyone
- **Per-test limits:** timeout, memory, CPU and network access, capped by server-wide maximums
- **Env vars and secrets:** secrets encrypted at rest with AES-256-GCM, write-only through the API, and redacted from logs
- **Logs and screenshots:** timestamped stdout/stderr per run; Playwright screenshots are collected automatically
- **Worker pool:** runs jobs in parallel with bounded concurrency, drains gracefully on shutdown, and sweeps stale jobs
- **Retention:** an hourly sweep prunes old runs, logs and screenshots
- **Auth:** optional Supabase sign-in protects every write route
- **Storage:** MinIO (or any S3-compatible store) or Supabase Storage
- **Web UI:** landing page at `/` and dashboard at `/app`

---

## The problem CI/CD doesn't solve

**"I can just run my Playwright tests in CI on a schedule."** That's the first objection. GitHub Actions cron jobs exist, so why BYOT?

**1. CI runs on code changes. Production breaks between deploys.**

```
Monday  9am  — deploy ships ✅
Monday  3pm  — Stripe silently changes their API
Monday 11pm  — payments are broken ❌
Tuesday 9am  — customer complains
```

Your CI never ran, so nobody knew. BYOT runs continuously, whether or not you deploy.

**2. CI tests your code. BYOT tests your dependencies.** A third-party API changes its response format, your CDN goes down, the database runs out of connections, an SSL certificate expires. None of these show up in CI. All of them show up in BYOT.

**3. CI is for developers. BYOT is for the business.**

```
CI/CD  →  "did my code break?"
BYOT   →  "is my checkout working right now?"
```

**4. Flaky tests kill CI adoption.** BYOT's consecutive-failure threshold means flaky tests don't cause noise, so teams actually keep them running.

### How it compares

Datadog Synthetics and Checkly are mature hosted products, and Checkly in particular runs Playwright tests well. BYOT is for teams that want the same idea fully open source, on their own servers, and across more test runtimes.

| | BYOT | Datadog Synthetics | Checkly |
|---|---|---|---|
| Open source (MIT) | ✓ | ✗ | CLI only |
| Runs entirely on your infrastructure | ✓ | private locations, hosted control plane | private locations, hosted control plane |
| Runtimes | Playwright, k6, Locust, Node, Python, Go binaries | Datadog's own browser and API tests | Playwright, API checks |
| Pricing | free, your hardware | per test run | per check run |
| Managed service, global locations, SLAs | ✗ | ✓ | ✓ |

---

## Quick Start

**Prerequisites:** Docker, Docker Compose and Go 1.26+

```bash
# 1. Start Postgres (5432) and MinIO (9000, console 9001)
docker compose up -d

# 2. Configure (the defaults work with docker compose)
cp .env.example .env

# 3. Pre-pull the runtime images (optional, avoids a slow first run)
go run . pull-images

# 4. Start the server on :3000 (database migrations run automatically)
go run .
```

Open http://localhost:3000/app for the dashboard, then upload one of the ready-made tests in [examples/](examples/) (a k6 API check, a Locust load test and a Playwright browser flow) to see a run end to end.

---

## Configuration

All settings come from environment variables or `.env`. See [.env.example](.env.example) for the full annotated list.

| Variable | Default | Description |
|---|---|---|
| `STORAGE_PROVIDER` | `minio` | `minio` or `supabase` |
| `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET`, `MINIO_REGION` | | MinIO / S3 settings |
| `SUPABASE_URL`, `SUPABASE_KEY`, `SUPABASE_BUCKET` | | Supabase Storage (`SUPABASE_KEY` is the `sb_secret_...` key) |
| `SUPABASE_PUBLISHABLE_KEY` | empty | `sb_publishable_...` key. With `SUPABASE_URL` set, it enables sign-in on write routes. Leave it empty to disable auth |
| `DB_URL` | | Postgres connection string |
| `SECRETS_KEY` | empty | Base64 32-byte key for secret env vars (`openssl rand -base64 32`). Back it up |
| `WORKER_CONCURRENCY` | `3` | Parallel jobs per instance (max 32) |
| `RETENTION_MAX_RUNS_PER_TEST` | `100` | Runs kept per test. `0` disables the limit |
| `RETENTION_DAYS` | `30` | Maximum run age. `0` disables the limit |
| `MAX_DOCKER_MEMORY_MB` | `2048` | Upper bound for per-test memory |
| `MAX_DOCKER_CPUS` | `4` | Upper bound for per-test CPUs |
| `MAX_TIMEOUT_SECONDS` | `900` | Upper bound for per-test timeouts |
| `AUTO_MIGRATE` | `true` | Apply database migrations on startup. Set `false` to run them yourself with the [`migrate`](https://github.com/golang-migrate/migrate) CLI |

---

## Runtimes

| Runtime | Image | Default command | Network |
|---|---|---|---|
| `node` | `node:24-alpine` | `node ./artifact` | off |
| `python` | `python:3.12-alpine` | `python ./artifact` | off |
| `go` | `ubuntu:24.04` (glibc, so CGO binaries run) | `./artifact` | off |
| `k6` | `grafana/k6:2.3.0` | `k6 run ./artifact` | on |
| `locust` | `locustio/locust:2.46.7` | `sh /app/.byot/run.sh` (see below) | on |
| `playwright` | `mcr.microsoft.com/playwright:v1.49.1-jammy` | `npx playwright test` | on |

### Locust

Upload a single locustfile (`.py`) or a `.zip` bundle with `locustfile.py` at its root (a single wrapper folder is fine). A bundle can include helper modules, data files, a `locust.conf` and a `requirements.txt`, which is installed before the run. Locust itself is already in the image.

The run is headless and configured with Locust's own env vars (or the bundle's `locust.conf`):

| Variable | Default | Meaning |
|---|---|---|
| `LOCUST_HOST` | none (required unless the User class sets `host`) | Base URL to test |
| `LOCUST_USERS` | `5` | Peak users |
| `LOCUST_SPAWN_RATE` | `1` | Users started per second |
| `LOCUST_RUN_TIME` | up to `30s`, whatever fits in the test timeout | How long to run, e.g. `2m` |

A run time that doesn't fit in the test timeout fails straight away with the timeout you need, so raise the timeout for longer runs (e.g. `LOCUST_RUN_TIME=5m` needs a timeout of about 310s).

A run fails if any request fails. Optional thresholds turn it into an SLO check:

| Variable | Fails the run when |
|---|---|
| `LOCUST_MAX_P95_MS` | p95 response time is above this (ms) |
| `LOCUST_MAX_AVG_MS` | average response time is above this (ms) |
| `LOCUST_MAX_FAIL_RATIO` | failed-request ratio is above this, e.g. `0.01` for 1%. Setting it allows failures below the ratio |
| `LOCUST_MIN_RPS` | throughput is below this many requests per second |

The log shows a `[byot] PASS` / `FAIL` line per threshold. The same settings work as `locust.conf` keys (`max-p95-ms = 500`). To pass extra Locust flags, override the command, e.g. `sh /app/.byot/run.sh --tags smoke`.

Uploads are limited to 10 MB. Playwright suites are uploaded as a `.zip` bundle (up to 200 MB) that includes `node_modules`, with `@playwright/test` pinned to **1.49.1** to match the browser image. See [examples/playwright-todo/build-zip.sh](examples/playwright-todo/build-zip.sh) for an example.

---

## API

When auth is enabled, every non-GET route needs a Supabase bearer token, and `/tests/{id}/env` needs one for reads too.

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Health check |
| `POST` | `/uploadBinary` | Upload a test (multipart, see below) |
| `GET` | `/tests` | List tests with their last run |
| `GET` / `DELETE` | `/tests/{id}` | Get or delete a test |
| `PATCH` | `/tests/{id}/config` | Update command, timeout, cron, webhook, threshold, limits |
| `POST` | `/tests/{id}/schedule/start` · `/schedule/stop` | Enable or disable the schedule |
| `GET` | `/tests/{id}/runs` | Run history |
| `GET` / `PUT` | `/tests/{id}/env` | List or set env vars (`{name, value, is_secret}`) |
| `DELETE` | `/tests/{id}/env/{name}` | Remove an env var |
| `POST` | `/run` | Queue a run: `{"testId": "..."}` and get back `job_id` |
| `POST` | `/stop` | Cancel the queued and running jobs for `{"testId": "..."}` |
| `GET` | `/status/{jobId}` | Job status |
| `GET` | `/runs/{runId}/log` | Plain-text run log |
| `GET` | `/screenshots/{runId}` · `/screenshot/{id}` | List or download screenshots |
| `GET` / `POST` | `/scheduler/status` · `/scheduler/stop` · `/scheduler/start` | Global scheduler pause and resume |

### Upload fields

| Field | Required | Description |
|---|---|---|
| `binary` | yes | The test file (`.zip` for Playwright; `.py` or `.zip` for Locust) |
| `runtime` | yes | `node`, `python`, `go`, `k6`, `locust`, `playwright` |
| `name` | no | Display name (defaults to the filename) |
| `command` | no | Override the runtime's default command |
| `timeout` | no | Seconds (default 30, max `MAX_TIMEOUT_SECONDS`) |
| `cron` | no | Cron expression. Setting it enables the schedule |
| `webhook_url` | no | Slack-compatible webhook for alerts. API reads return it masked (`https://hooks.slack.com/…a1b2`) |
| `failure_threshold` | no | Consecutive failures before an alert fires |
| `docker_memory_mb`, `docker_cpus`, `docker_network_enabled` | no | Per-test resource limits |

```bash
curl -X POST http://localhost:3000/uploadBinary \
  -F "binary=@./checkout.zip" \
  -F "runtime=playwright" \
  -F "cron=*/15 * * * *" \
  -F "webhook_url=https://hooks.slack.com/services/..."
# → { "id": "a3f8b2c1-...", "message": "binary uploaded successfully" }

curl -X POST http://localhost:3000/run -d '{"testId":"a3f8b2c1-..."}'
# → { "job_id": "...", "status": "queued", "message": "job queued for execution" }
```

---

## Architecture

```
Browser / curl
     │
     ▼
HTTP server (:3000) ── Supabase auth (write routes)
     │
     ├─ /uploadBinary ──► Storage (MinIO / Supabase)
     ├─ /run ───────────► Postgres job queue ◄── Scheduler (cron, every 30s)
     │                          │
     │                          ▼
     │                    Worker pool (WORKER_CONCURRENCY)
     │                          │  download artifact, decrypt env
     │                          ▼
     │                    docker run (hardened, ephemeral, byot-<jobID>)
     │                          │
     │                          ├─► logs + screenshots ──► Storage
     │                          ├─► run result ──► Postgres
     │                          └─► webhook alert / recovery
     │
     └─ Retention sweeper (hourly)
```

---

## Security model

BYOT runs code you upload, so it is built to contain that code, not to run code from strangers.

- **Each run gets a fresh container** that is removed afterwards: all Linux capabilities dropped, `no-new-privileges`, read-only root filesystem, non-root user, 128-process limit, and memory, CPU and timeout caps.
- **Network is off by default** for `node`, `python` and `go`, and on for `k6`, `locust` and `playwright`, which need it. You can override it per test.
- **Secrets** are encrypted at rest with AES-256-GCM, never returned by the API, passed to containers by name (so they don't appear in the process list) and redacted from logs.
- **The Docker daemon is the trust boundary.** The server starts containers through the host's Docker, so anyone who can upload a test can run code on that host inside the sandbox above. Run BYOT on a dedicated VM, enable auth before exposing it, and only give accounts to people you would trust with that.
- **Auth** (Supabase) protects every write route. Read routes (test list, run history, logs, screenshots) stay open, so keep the server on a private network or behind a proxy if your logs are sensitive.

Found a vulnerability? Please open a private [security advisory](https://github.com/alexanderritik/BYOT/security/advisories/new) instead of a public issue.

## Development

```bash
docker compose up -d   # infra
air                    # hot reload (go install github.com/air-verse/air@latest)
go test ./...
```

## License

MIT, see [LICENSE](LICENSE).
