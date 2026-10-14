# BYOT — Bring Your Own Tests

> Monitor your production business flows, not just your code.

BYOT is an open-source production monitoring platform. It runs the tests you already have (Playwright, k6, Node, Python, Go binaries) on a schedule inside isolated Docker containers, and alerts you when something actually breaks.

## Demo

<video src="docs/launch.mp4" controls muted width="100%"></video>

▶️ **[Watch the launch video](docs/launch.mp4)** if the player above doesn't load.

---

## Features

- **Any test runtime:** `node`, `python`, `go`, `k6`, `playwright` (zip bundle)
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

### Why not Datadog or Checkly?

| Feature | BYOT | Datadog | Checkly |
|---|---|---|---|
| Use existing tests | ✓ | ✗ | ✗ |
| Self-hostable | ✓ | ✗ | ✗ |
| Open source | ✓ | ✗ | ✗ |
| Flaky test detection | ✓ | paid tier | limited |
| Custom runtimes | ✓ | ✗ | limited |

---

## Quick Start

**Prerequisites:** Docker, Docker Compose, Go 1.26+, and the [`migrate`](https://github.com/golang-migrate/migrate) CLI

```bash
# 1. Start Postgres (5432) and MinIO (9000, console 9001)
docker compose up -d

# 2. Configure (the defaults work with docker compose)
cp .env.example .env

# 3. Apply database migrations
migrate -path ./migrations -database "postgres://fnx:fnx@localhost:5432/fnx?sslmode=disable" up

# 4. Pre-pull the runtime images (optional, avoids a slow first run)
go run . pull-images

# 5. Start the server on :3000
go run .
```

Open http://localhost:3000/app for the dashboard.

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

---

## Runtimes

| Runtime | Image | Default command | Network |
|---|---|---|---|
| `node` | `node:18-alpine` | `node ./artifact` | off |
| `python` | `python:3.12-alpine` | `python ./artifact` | off |
| `go` | `ubuntu:24.04` (glibc, so CGO binaries run) | `./artifact` | off |
| `k6` | `grafana/k6:latest` | `k6 run ./artifact` | on |
| `playwright` | `mcr.microsoft.com/playwright:v1.49.1-jammy` | `npx playwright test` | on |

Uploads are limited to 10 MB. Playwright suites are uploaded as a `.zip` bundle (up to 200 MB) that includes `node_modules`; see [testfunction/encardio-playwright/build-zip.sh](testfunction/encardio-playwright/build-zip.sh) for an example.

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
| `binary` | yes | The test file (`.zip` for Playwright) |
| `runtime` | yes | `node`, `python`, `go`, `k6`, `playwright` |
| `name` | no | Display name (defaults to the filename) |
| `command` | no | Override the runtime's default command |
| `timeout` | no | Seconds (default 30) |
| `cron` | no | Cron expression. Setting it enables the schedule |
| `webhook_url` | no | Slack-compatible webhook for alerts |
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

## Development

```bash
docker compose up -d   # infra
air                    # hot reload (go install github.com/air-verse/air@latest)
go test ./...
```

## License

MIT, see [LICENSE](LICENSE).
