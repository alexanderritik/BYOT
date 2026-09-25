# BYOT — Implementation plan

## Product (what we sell vs how we build)

**Customer-facing (market):**

> Run the tests you already have on a schedule — without rebuilding them into a monitoring platform.

**Not the headline:**

> Upload your binary and run it.

Examples:

```text
Playwright tests → BYOT → every 15m → production checkout → screenshot on failure → Slack

k6 script        → BYOT → hourly     → API health        → alert if failed
```

**Implementation (engineering):** upload artifact → runtime registry (image + command + defaults) → workspace → Docker sandbox → logs + artifacts → alerts.

---

## North-star architecture (long term)

```text
                         BYOT
                          │
              ┌───────────┴───────────┐
              │                       │
          Control Plane          Execution Plane
              │                       │
       API / Scheduler              Queue
       Tests / Alerts                 │
              │             ┌─────────┼─────────┐
              │             ▼         ▼         ▼
              │          Mumbai   Singapore  Frankfurt   ← AFTER first users
              │             │         │         │
              │          Sandbox   Sandbox   Sandbox
              │             │         │         │
              └─────────────┴─────────┴─────────┘
                            │
                         Results
                            │
                 ┌──────────┴──────────┐
                 ▼                     ▼
             Postgres                MinIO
                 │                     │
                 └──────────┬──────────┘
                            ▼
                       Alert Engine
```

**Principle:** Control plane decides *when* and *what*; execution plane runs *how*. Scheduler only enqueues; workers use runtime registry only.

**Risk to avoid:** 2–3 months of infrastructure polish before learning if anyone will pay. Optimize for **first external users**, not P5.

---

## “SHIP THIS” gate (market-ready MVP)

Verified against repo on **2026-09-25** (`go build ./...` OK):

| Capability | Status | Notes |
|------------|--------|--------|
| Docker sandbox + worker | ✅ | `runtime.Execute`, `worker.executeJob` |
| Queue + jobs | ✅ | `FOR UPDATE SKIP LOCKED` dequeue |
| Go / Node runtimes | ✅ | E2E via `testfunc/` |
| Python registry | ✅ | Not E2E-validated |
| k6 registry | ⚠️ | In `runtimes` map; **no E2E** |
| Playwright | ❌ | No bundle unpack / image |
| Scheduler | ✅ | Goroutine in `main`, cron on upload (`cron` form field) |
| Network policy (registry) | ✅ | `NetworkEnabled`; k6/playwright → `bridge`, go/node → `none` |
| Logs → MinIO | ✅ | Per run; timestamps UTC in executor |
| Screenshots / traces | ❌ | UI mock in `product.html` only |
| Slack / webhook | ❌ | UI mock only |
| Basic dashboard | ⚠️ | Static `product.html` / `index.html`, not wired to API |
| Network for monitoring tests | ✅ | Per-runtime `NetworkEnabled` in registry |

```text
                ┌─────────────────────────┐
                │       SHIP THIS         │
                │  (then talk to users)   │
                └─────────────────────────┘
                           │
         ┌─────────────────┼─────────────────┐
         ▼                 ▼                 ▼
   ✓ queue/worker    ○ scheduler      ○ Playwright
   ✓ go/node         ○ k6 E2E         ○ artifacts
   ○ network policy  ○ Slack          ○ dashboard↔API
                           │
                           ▼
                    TALK TO USERS
                           │
                 ┌─────────┴─────────┐
                 ▼                   ▼
            They love it         They don't
                 │                   │
                 ▼                   ▼
         multi-region, HA,      reposition /
         billing, secrets       narrow ICP
```

**Milestone that matters more than “P5 complete”:**

> **Three people outside my machine** have a **real production test** running on BYOT and receive a **useful failure alert**.

---

## Not required for market-ready (defer aggressively)

- ❌ Multi-region (Mumbai / Singapore / Frankfurt)
- ❌ Quorum across regions, round-robin geo scheduling
- ❌ HA scheduler / `pg_try_advisory_lock` (fine **later**)
- ❌ Worker fleet orchestration as a product surface
- ❌ Enterprise PagerDuty / on-call rotations
- ❌ Perfect sandbox (cap-drop, read-only root, etc.) before users
- ❌ Kafka / Temporal / K8s for cron
- ❌ Every runtime polished equally

**Scheduler v1 is enough:**

```text
single API process → scheduler goroutine → Postgres → queue.Enqueue
```

---

## Phasing (reordered for market, not infra depth)

```text
                    ┌── k6 E2E (+ network ON)
                    │
P0 network + sandbox ─┼──► Scheduler ──► Playwright bundle
    (minimal)       │                           │
                    │                           ▼
                    │                    Artifacts (screenshots)
                    │                           │
                    │                           ▼
                    │                        Alerts (Slack)
                    │                           │
                    │                           ▼
                    │                     FIRST USERS  ← P0.5
                    │                           │
                    └───────────────────────────┴──► Multi-region (P5) much later
```

**Key change:** Playwright + artifacts + alerts **before** deep sandbox polish and **long before** multi-region.

---

## MVP runtime matrix

One registry (`image` + `command` + `network_enabled` + defaults). Don’t build seven frameworks — prove **Playwright + k6** for the story; others show BYOT isn’t locked in.

| Runtime | MVP ship | Role |
|---------|----------|------|
| **Playwright** | **YES** | Primary customer story |
| **k6** | **YES** | API / load health story |
| Node | YES | Demos + simple scripts |
| Python | YES | Same |
| Go / custom binary | YES-ish | Already works (`testfunc`) |
| curl / generic | Later | |

---

## P0 — Sandbox + network (minimal, user-blocking)

**Why now:** Playwright and k6 **must hit the internet**. `--network=none` is correct for untrusted compute-only artifacts; **wrong default for monitoring**.

```text
                 Test
                   │
          ┌────────┴────────┐
          │                 │
    network OFF         network ON
          │                 │
   local / compute      production monitoring
                              │
                    (later: egress policy, DNS, secrets)
```

- [x] `RuntimeSpec.NetworkEnabled` (registry: `true` for `playwright`, `k6`; `false` for `go` unless overridden)
- [x] Docker: `bridge` when enabled; `none` otherwise (`runtime/docker_args.go`)
- [ ] Document trust model: user opts into network per runtime / test
- [ ] **Defer:** destination allowlists, DNS policy, bandwidth caps, secret injection
- [ ] Persist `exit_code` on `tests_runs` (small, high value)
- [ ] **Defer:** cap-drop, cidfile kill, log streaming — until after first users

**Go learning:** extend struct + table-driven test for docker args; don’t special-case in worker beyond reading spec.

---

## P0.5 — Customer validation (parallel with build)

Not magic numbers — distinguish **technically cool** from **painful enough to pay**.

- [ ] **10** engineers run a **real** test (not hello-world)
- [ ] **5** schedule it (cron)
- [ ] **3** connect Slack/webhook
- [ ] **3** keep it running **1 week**
- [ ] Ask: what would you **replace** (Actions cron, Checkly, DIY)?
- [ ] Ask: what would you **pay** for?
- [ ] **1** paid pilot (even small)

Run this **while** finishing Playwright + artifacts + alerts — don’t wait for P5.

---

## P1 — Scheduler (control plane)

**Goal:** Cron → `Queue.Enqueue(testID)` only.

### Data model (`tests`)

- [ ] `schedule_cron`, `schedule_enabled`, `next_run_at` (index); cron interpreted in UTC
- [ ] `concurrency_policy` default `forbid`

### Behavior

- [x] `scheduler` package in **same process** as API; tick 30s UTC
- [x] Forbid overlap: no enqueue if `queued`/`running` job for `test_id`
- [x] Advance `next_run_at` each tick (skip backfill storm)
- [x] Upload: optional `cron`; compute `next_run_at` (UTC)
- [x] `jobs.trigger`: `manual` | `scheduled`
- [ ] **Later:** `pg_try_advisory_lock` for multi-replica API

### API

- [ ] Schedule on upload + `GET/PATCH` test (schedule, `next_run_at`)

---

## P1b — k6 E2E

- [ ] `testfunc` k6 script + upload + run (manual then scheduled)
- [ ] Network ON via registry
- [ ] Document: artifact file + `k6 run ./artifact`

---

## P2 — Playwright (project bundle)

```text
upload zip → workspace unpack → npx playwright test (or Test.Command)
```

- [ ] Zip upload + safe unpack (zip slip)
- [ ] Registry: Playwright image + default command + network ON
- [ ] Prefer **bundled `node_modules`** in zip for MVP (no `npm install` every run)
- [ ] Template `playwright.config` paths for screenshots/traces under workspace

---

## P3 — Artifacts (screenshots, traces)

**Target UX (what we ship toward):**

```text
checkout.spec.ts  ● Failing
Last run 2m ago · Expected "Payment successful" · Received "Payment failed"
[ View logs ]  Artifacts: screenshot.png, trace.zip
Schedule: every 15m · Alert: Slack ✓  [ Run now ]
```

- [ ] `run_artifacts` table + MinIO `{test-id}/runs/{run-id}/...`
- [ ] Worker: upload `test-results/`, `screenshots/` after run
- [ ] API: list artifacts for a run
- [ ] Wire **product.html** (or minimal real UI) to API for status, logs, artifacts

---

## P4 — Alerts (minimal)

- [ ] `webhook_url`, `failure_threshold`, consecutive failure counter on test
- [ ] Fire Slack-compatible webhook when threshold crossed; dedupe repeats
- [ ] Optional “recovered” message on pass after alert
- [ ] **Not now:** PagerDuty, routing rules, on-call

---

## P5 — Multi-region (after validation)

Only if users ask for geo vantage — default customer pain is **“checkout broke”**, not **“run in Frankfurt.”**

- [ ] Workers with `BYOT_REGION`; enqueue per region or run groups
- [ ] Quorum / parallel policies (Checkly-style) — **much later**

---

## Suggested sprints (market-first)

| Sprint | Focus | Exit criteria |
|--------|--------|----------------|
| **A** | P0 network + P1 scheduler | Cron enqueue works; k6 can hit a URL |
| **B** | P1b k6 + P2 Playwright | One real Playwright zip runs in Docker |
| **C** | P3 artifacts + P4 Slack + dashboard API | Failure → screenshot in MinIO → webhook fires |
| **D** | P0.5 outreach | 3 external users on real tests for 1 week |

---

## Anti-goals (for now)

- Building distributed multi-region before **3 external users**
- Competing with Datadog on full observability
- Exactly-once scheduling
- Arbitrary user shell without sandbox review

---

## Open decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | Cron only vs `every 15m` presets in UI? | TBD (presets help non-cron users) |
| 2 | Overlap forbid: skip slot vs queue after finish? | TBD |
| 3 | Playwright zip: require bundled `node_modules` for MVP? | **Recommend yes** |
| 4 | Dashboard: extend `product.html` vs small Go templates? | TBD |
| 5 | First paid tier: self-host free + hosted $? | TBD after P0.5 interviews |

---

## Learnings captured (from market + architecture review)

| Learning | Plan change |
|----------|-------------|
| Biggest unknown is **payment / retention**, not executor design | Added **P0.5** and **SHIP THIS** gate before P5 |
| Customers buy **scheduled existing tests + alert**, not “binary upload” | **Product** section at top; README/marketing should match |
| Playwright/k6 need **network** | P0 reprioritized **network policy** over deep hardening |
| Multi-region is **sales rarely ask first** | P5 explicitly **after FIRST USERS** |
| Scheduler HA/K8s is overkill for MVP | Single goroutine + Postgres; lock only later |
| Registry scales runtimes | MVP matrix: **Playwright + k6** hero; others proof of generality |
| Dashboard mock ahead of backend | P3 includes **wire UI to API** |
| Core loop already works | Don’t rewrite — **enqueue path** stays; add schedule + artifacts + webhooks |

---

## Engineering reference (unchanged core)

**Control plane:** API, scheduler, tests config, alerts.  
**Execution plane:** queue, worker, workspace, `runtime.Execute`, MinIO results.  
**Go habits:** `defer` cleanup, `context` timeout, `(T, bool)` registry lookup, infra vs test failure split in `Execute`.
