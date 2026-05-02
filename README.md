# BYOT — Bring Your Own Tests

> Monitor your production business flows, not just your code.

BYOT is an open-source production monitoring platform that executes your existing test binaries — Playwright, k6, custom scripts, anything — on a schedule inside isolated Docker containers, and alerts you when something actually breaks.

---

## The problem CI/CD doesn't solve

**"I can just run my Playwright tests in CI on a schedule"** — that's the first objection. GitHub Actions cron jobs exist. So why BYOT?

**1. CI runs on code changes. Production breaks between deploys.**

```
Monday  9am  — deploy ships ✅
Monday  3pm  — Stripe silently changes their API
Monday 11pm  — payments are broken ❌
Tuesday 9am  — customer complains
```

Your CI never ran. Your tests never ran. Nobody knew. BYOT runs continuously, regardless of deploys.

**2. CI tests your code. BYOT tests your dependencies.**

Your code didn't change. But:
- A third-party API changed their response format
- Your CDN went down
- The database ran out of connections
- An SSL certificate expired
- An external payment provider had an outage

None of these show up in CI. All of them show up in BYOT.

**3. CI is for developers. BYOT is for the business.**

```
CI/CD  →  "did my code break?"
BYOT   →  "is my checkout working right now?"
```

Product managers, founders, and support teams care about the second question. They can't read CI logs. BYOT gives them a green/red answer.

**4. Flaky tests kill CI adoption.**

Teams disable E2E tests in CI because flaky tests block deploys. Those tests rot. BYOT's consecutive-failure threshold means flaky tests don't cause noise — so teams actually keep them running.

**5. One incident pays for itself.**

One missed P0 = hours of engineer time + customer churn. If BYOT catches one checkout failure before a customer does, it paid for itself 10× over.

---

## Who it's for

Not solo developers. Not teams with one service.

**Target:** Teams with 5+ engineers, multiple services, external dependencies, and an existing E2E test suite that's gathered dust because "it's too flaky for CI." That's a real market, and they get immediate value.

---

## Why not Datadog or Checkly?

| Feature | BYOT | Datadog | Checkly |
|---|---|---|---|
| Use existing tests | ✓ | ✗ | ✗ |
| Self-hostable | ✓ | ✗ | ✗ |
| Open source | ✓ | ✗ | ✗ |
| Flaky test detection | ✓ | paid tier | limited |
| Custom runtimes | ✓ | ✗ | limited |
| Free tier | unlimited | 5 tests | 3 tests |

Datadog does this — at $500+/month with a proprietary DSL you have to learn. BYOT does it for $29 or free, with the tests you already wrote.

---

## Quick Start

**Prerequisites:** Docker, Docker Compose, Go 1.21+

### 1. Start the infrastructure

```bash
docker compose up -d
```

This starts PostgreSQL (port 5432) and MinIO (port 9000 / console 9001).

### 2. Configure environment

```bash
cp .env.example .env
# Edit .env if needed — defaults work out of the box with docker compose
```

### 3. Run the server

```bash
go run .
# Starts on :3000 — migrations run automatically
```

---

## API

### Health check

```
GET /health
→ { "status": "ok" }
```

### Upload a test binary

```
POST /uploadBinary
Content-Type: multipart/form-data
```

| Field | Type | Required | Description |
|---|---|---|---|
| `binary` | file | yes | Compiled test binary |
| `runtime` | string | yes | `go` or `node` |
| `severity` | string | yes | `P0` `P1` `P2` `P3` |

```bash
curl -X POST http://localhost:3000/uploadBinary \
  -F "binary=@./e2e/checkout.test" \
  -F "runtime=node" \
  -F "severity=P0"

# → { "id": "a3f8b2c1-...", "message": "binary uploaded successfully" }
```

### Run a test

```
POST /run/{id}
Content-Type: application/json

{ "filename": "a3f8b2c1-...", "runtime": "node", "timeout": 60 }
```

Output is stdout/stderr from the container, timestamped per line:

```
2026-05-03T10:30:00Z [stdout] ✓ Login successful
2026-05-03T10:30:04Z [stdout] ✓ Cart updated
2026-05-03T10:30:06Z [stderr] ✗ Pay button not found
```

---

## Severity levels

| Level | Meaning |
|---|---|
| P0 | Wake me up at 3am — production is down |
| P1 | Critical path broken, escalate fast |
| P2 | Degraded, alert during business hours *(default)* |
| P3 | Nice to know — weekly digest |

---

## Runtimes

| Runtime | Docker image | Notes |
|---|---|---|
| `go` | `alpine` | Executes compiled binary directly |
| `node` | `node:18` | Runs with `node` interpreter |

More runtimes (Python, Ruby, Deno) are planned — contributions welcome.

---

## Architecture

```
Client
  │
  ▼
HTTP Server (:3000)
  │
  ├─ POST /uploadBinary ──► MinIO  ({uuid}/binary)
  │
  └─ POST /run/{id}
       │
       ├─ Download binary from MinIO
       ├─ docker run --rm -v /tmp/...:/app/binary <image>
       ├─ Stream stdout + stderr (RFC3339 timestamps)
       └─ Upload logs to MinIO  ({uuid}/logs/{timestamp}.txt)

PostgreSQL
  ├─ tests       (uuid, runtime, severity, binary_url, created_at)
  └─ tests_runs  (uuid, test_id, status, duration_ms, log_url, ...)
```

Each execution runs in a **fresh, ephemeral Docker container** — no shared state between runs.

---

## Self-hosting

The full stack is a single `docker compose up`. For production:

- Point `DB_URL` at a managed Postgres (RDS, Supabase, Neon, etc.)
- Point MinIO config at S3 or any S3-compatible store
- Run the Go binary behind a reverse proxy (nginx, Caddy)

---

## Development

```bash
docker compose up -d  # start infra
air                   # hot reload (go install github.com/air-verse/air@latest)
go test ./...
```

---

## License

MIT — see [LICENSE](LICENSE).
