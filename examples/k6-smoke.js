// BYOT example: k6 API smoke check.
// Runs in about 10s, well inside BYOT's default 30s timeout. k6 exits non-zero
// when a threshold fails, so BYOT marks the run failed and can alert.
//
// Upload: runtime=k6, file=k6-smoke.js
// Point it at your own API with the BASE_URL env var.
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'https://fakestoreapi.com';

export const options = {
  vus: 2,
  duration: '10s',
  thresholds: {
    checks: ['rate==1.0'],            // every check must pass
    http_req_failed: ['rate<0.01'],   // under 1% errors
    http_req_duration: ['p(95)<2000'], // p95 under 2s
  },
};

export default function () {
  const list = http.get(`${BASE_URL}/products?limit=5`);
  check(list, {
    'catalog returns 200': (r) => r.status === 200,
    'catalog has products': (r) => Array.isArray(r.json()) && r.json().length > 0,
  });

  const item = http.get(`${BASE_URL}/products/1`);
  check(item, {
    'product returns 200': (r) => r.status === 200,
    'product has a price': (r) => typeof r.json('price') === 'number',
  });

  sleep(1);
}
