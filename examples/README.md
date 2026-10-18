# Examples

Three ready-to-upload tests. Each one passes against a public demo target, so you can see a green run, logs and (for Playwright) a screenshot within a minute of starting BYOT.

| File | Runtime | What it checks | Default timeout OK? |
|---|---|---|---|
| [`k6-smoke.js`](k6-smoke.js) | `k6` | Product API returns data; p95 < 2s, < 1% errors | yes (~10s) |
| [`locustfile.py`](locustfile.py) | `locust` | 5 shoppers browsing a catalog; any failed request fails the run | yes (~30s) |
| [`playwright-todo.zip`](playwright-todo.zip) | `playwright` | Adds and completes a todo in a real browser, saves a screenshot | yes (~10s) |

## Upload from the dashboard

Open http://localhost:3000/app, choose **+ Upload test**, pick the runtime from the table and upload the file.

## Upload with curl

```bash
curl -X POST http://localhost:3000/uploadBinary -F runtime=k6         -F binary=@examples/k6-smoke.js
curl -X POST http://localhost:3000/uploadBinary -F runtime=locust     -F binary=@examples/locustfile.py
curl -X POST http://localhost:3000/uploadBinary -F runtime=playwright -F binary=@examples/playwright-todo.zip

# then run one
curl -X POST http://localhost:3000/run -d '{"testId":"<id from the upload>"}'
```

Add `-F "cron=*/5 * * * *"` to run on a schedule, and `-F webhook_url=...` for Slack alerts. With auth enabled, add `-H "Authorization: Bearer <token>"`.

## Make them yours

- **k6:** set the `BASE_URL` env var on the test to point at your API.
- **Locust:** set `LOCUST_HOST`, and optionally `LOCUST_MAX_P95_MS` to turn it into an SLO check.
- **Playwright:** edit [`playwright-todo/tests/todo.spec.ts`](playwright-todo/tests/todo.spec.ts) and rebuild with `./playwright-todo/build-zip.sh`. The bundle includes `node_modules`, and `@playwright/test` must stay at **1.49.1** to match BYOT's browser image.

Only load-test services you own or that are published for testing.
