# MVP 1 — checklist

**Goal:** a team can upload a test, give it secrets, schedule it, and get alerted when it breaks, on a deployed instance.

Scope = Phase 1 (core execution) + scheduling, webhooks, limits, timeouts + env vars/secrets + auth on upload and env.

## Done
- Docker execution: node, python, go, k6, playwright
- Logs, run/job status, Playwright screenshots
- Cron scheduling, webhooks with failure threshold + recovery
- Per-test timeout and resource limits (memory, CPU, network)
- Sandbox hardening: `--cap-drop=ALL`, `no-new-privileges`, `--read-only`, tmpfs `/tmp`, non-root `--user`
- Storage: MinIO and Supabase (`storage/`)
- Auth: Supabase sign-in (publishable key). Every non-GET route requires it (`RequireWrites` wraps all API routes); env endpoints require it for reads too. Other GETs (dashboard, runs, logs, screenshots) are public.
- Env vars + secrets: AES-256-GCM (`SECRETS_KEY`), write-only secrets, log redaction, migration 013
- Runtime verification with hardening flags: confirmed by the user (all containers run as usual)
- Repo cleanup: build binary untracked

## Decisions
- Migrations run automatically at startup (`runMigrations` in `main.go`), so migration 013 applies on next deploy.
- Env vars cannot be set at upload time; add them on the test page after upload.
- Supabase keys: `sb_publishable_...` for auth (`SUPABASE_PUBLISHABLE_KEY`), `sb_secret_...` for storage (`SUPABASE_KEY`). No legacy anon key.

## Remaining (priority order)
1. End-to-end test on the deployed box: upload, schedule, fail on purpose, webhook, recovery
2. Worker concurrency (pool of 3-5; dequeue must use `FOR UPDATE SKIP LOCKED`)
3. Stuck-job recovery: on startup, mark stale `running` jobs failed
4. Generic artifacts (collect all files from an output dir; last open Phase 1 item)
5. Run retention (cap runs, logs, screenshots)
6. Go runtime: alpine/musl breaks CGO binaries; use a Debian image or require `CGO_ENABLED=0`
7. Decide on open sign-up (anyone can create an account and upload code) — disable sign-ups in Supabase or add an allowlist

## After MVP 1
Playground, JUnit/TAP parsing, videos/traces, teams/billing, DAGs, multi-worker, Kubernetes.
