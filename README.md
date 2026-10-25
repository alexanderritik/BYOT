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
- **Alerts:** PagerDuty (one incident per test, resolved automatically on recovery) and Slack-compatible webhooks, with a consecutive-failure threshold so flaky tests don't page anyone
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

**Prerequisites:** Docker with Compose. Nothing else.

```bash
curl -O https://raw.githubusercontent.com/alexanderritik/BYOT/main/docker-compose.yml
docker compose up -d
```

That starts BYOT (the published image `ghcr.io/alexanderritik/byot`), Postgres and MinIO. Database migrations run on boot and the storage bucket is created automatically.

Open http://localhost:3000/app for the dashboard, then upload one of the ready-made tests in [examples/](examples/) (a k6 API check, a Locust load test and a Playwright browser flow) to see a run end to end. The first run of each runtime pulls its image, so it takes longer. To pull them all up front:

```bash
docker compose exec byot /app/mini-lambda pull-images
```

> **How tests run:** BYOT mounts the host's Docker socket and starts each test as a sibling container on the host (hardened as listed under Features, as user `nobody`). Access to the Docker socket is root-equivalent on that host, so run BYOT on a machine dedicated to it. Run workspaces live in `/tmp/byot-runs` on the host.

That setup uses default passwords and no sign-in, which is fine on your laptop. For a server, see [Running in production](#running-in-production).

---

## Running in production

About five minutes on any Linux host with Docker. Use a VM dedicated to BYOT (see the note above about the Docker socket).

**1. Download the compose file and generate credentials**

```bash
mkdir byot && cd byot
curl -O https://raw.githubusercontent.com/alexanderritik/BYOT/main/docker-compose.yml

cat > .env <<EOF
BYOT_VERSION=0.1.0
POSTGRES_PASSWORD=$(openssl rand -hex 24)
MINIO_ROOT_PASSWORD=$(openssl rand -hex 24)
SECRETS_KEY=$(openssl rand -base64 32)
EOF
chmod 600 .env
```

Docker Compose reads `.env` automatically. Pinning `BYOT_VERSION` means a restart never pulls a surprise upgrade. Set the passwords **before the first start**: Postgres only applies its password when it creates the database volume.

**2. Start it**

```bash
docker compose up -d
docker compose exec byot /app/mini-lambda pull-images   # optional, makes first runs fast
curl -s localhost:3000/health
```

Postgres and MinIO listen on `127.0.0.1` only; BYOT reaches them over the compose network.

**3. Control who can reach it.** Anyone who can upload a test can run code on this host, so pick one:

- **Private network (fastest):** keep port 3000 reachable only from your VPN, VPC or Tailscale network, and close it in the firewall everywhere else.
- **Public URL:** turn on sign-in and put TLS in front. Create a [Supabase](https://supabase.com) project and add its URL and publishable key to `.env`:

  ```bash
  SUPABASE_URL=https://your-project-ref.supabase.co
  SUPABASE_PUBLISHABLE_KEY=sb_publishable_...
  BYOT_LISTEN=127.0.0.1:3000          # only the proxy can reach BYOT
  ```

  Then use any reverse proxy for HTTPS. With [Caddy](https://caddyserver.com) on the host, this `Caddyfile` gets a certificate automatically:

  ```
  byot.example.com {
      reverse_proxy localhost:3000
  }
  ```

  Run `docker compose up -d` again to apply the changes. Sign-in protects every write route. Read routes (test list, run history, logs) stay open, so use the private-network option if your logs are sensitive.

**Back up** the `postgres_data` and `minio_data` volumes, and keep a copy of `SECRETS_KEY` somewhere safe: without it, stored secrets can't be decrypted.

**Upgrade** by changing `BYOT_VERSION` in `.env`, then:

```bash
docker compose pull && docker compose up -d
```

Migrations run on startup. Images are tagged `X.Y.Z`, `X.Y` and `X` (releases), `latest` (newest stable release), `edge` (every push to main) and `sha-<commit>`.

---

## Development

**Prerequisites:** Docker, Docker Compose and Go 1.26+

```bash
# 1. Start only Postgres (5432) and MinIO (9000, console 9001)
docker compose up -d postgres minio

# 2. Configure (the defaults work with docker compose)
cp .env.example .env

# 3. Start the server on :3000 (database migrations run automatically)
go run .
```

For hot reload use [air](https://github.com/air-verse/air) (`go install github.com/air-verse/air@latest`, then `air`), and run tests with `go test ./...`.

`docker compose up -d --build` builds the image from your checkout instead of pulling it.

### Releasing

Actions → **release** → Run workflow → enter a version (`1.2.0`, or `1.3.0-rc.1` for a pre-release). Or push a tag: `git tag v1.2.0 && git push origin v1.2.0`. Either way the workflow tests, tags, pushes the multi-arch image and publishes a GitHub Release with generated notes.

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
| `PAGERDUTY_EVENTS_URL` | US endpoint | Set to `https://events.eu.pagerduty.com/v2/enqueue` for PagerDuty EU accounts |
| `WORKSPACE_DIR` | `/tmp` | Where run workspaces are created. When BYOT itself runs in a container, this must be bind-mounted at the same path on the host (the compose file uses `/tmp/byot-runs`) |
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
| `PATCH` | `/tests/{id}/config` | Update command, timeout, cron, webhook, PagerDuty key, threshold, limits |
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
| `pagerduty_routing_key` | no | PagerDuty Events API v2 integration key (32 characters). Opens an incident at the failure threshold and resolves it when the test passes. API reads return it masked (`…a1b2`) |
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
     │                          └─► PagerDuty / webhook alert, recovery
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

## License

MIT, see [LICENSE](LICENSE).
