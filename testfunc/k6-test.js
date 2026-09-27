import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Real public HTTP testing service.
// Use a production website only when you have explicit authorization.
const BASE_URL = 'https://httpbin.org';

// Custom metrics
const failedRequests = new Rate('failed_requests');
const checkoutDuration = new Trend('checkout_duration');
const cartRequests = new Counter('cart_requests');

export const options = {
  scenarios: {
    // Normal traffic
    normal_load: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '30s', target: 5 },
        { duration: '1m', target: 10 },
        { duration: '2m', target: 10 },
        { duration: '30s', target: 0 },
      ],
      gracefulRampDown: '10s',
      exec: 'userJourney',
    },

    // Short burst of traffic
    traffic_spike: {
      executor: 'ramping-vus',
      startTime: '1m',
      startVUs: 0,
      stages: [
        { duration: '15s', target: 20 },
        { duration: '30s', target: 20 },
        { duration: '15s', target: 0 },
      ],
      gracefulRampDown: '10s',
      exec: 'userJourney',
    },

    // Lightweight background API monitoring
    api_monitoring: {
      executor: 'constant-vus',
      vus: 2,
      duration: '4m',
      exec: 'apiHealthCheck',
    },
  },

  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<1500', 'p(99)<3000'],

    failed_requests: ['rate<0.05'],
    checkout_duration: ['p(95)<2000'],

    checks: ['rate>0.95'],
  },

  // Useful tags for filtering results
  tags: {
    test_type: 'complex-load-test',
    environment: 'test',
  },
};

// Generate a random product ID
function randomProductId() {
  return Math.floor(Math.random() * 1000) + 1;
}

// Generate a random customer
function randomCustomer() {
  return {
    customerId: `customer-${__VU}-${__ITER}`,
    productId: randomProductId(),
    quantity: Math.floor(Math.random() * 3) + 1,
  };
}

// ----------------------------------------------------
// Main user journey
// ----------------------------------------------------

export function userJourney() {
  const customer = randomCustomer();

  group('01 - Homepage / Landing Page', function () {
    const res = http.get(`${BASE_URL}/get`, {
      tags: {
        endpoint: 'homepage',
      },
    });

    const success = check(res, {
      'homepage status is 200': (r) => r.status === 200,
      'homepage returns JSON': (r) =>
        r.headers['Content-Type']?.includes('application/json'),
    });

    failedRequests.add(!success);
  });

  sleep(Math.random() * 2 + 1);

  // --------------------------------------------------
  // Product browsing
  // --------------------------------------------------

  group('02 - Browse Product', function () {
    const res = http.get(
      `${BASE_URL}/anything/products/${customer.productId}`,
      {
        tags: {
          endpoint: 'product',
        },
      }
    );

    const success = check(res, {
      'product request succeeded': (r) => r.status === 200,
      'product response contains URL': (r) =>
        r.json('url') !== undefined,
    });

    failedRequests.add(!success);
  });

  sleep(Math.random() * 2 + 1);

  // --------------------------------------------------
  // Search/filter operation
  // --------------------------------------------------

  group('03 - Product Search', function () {
    const params = {
      tags: {
        endpoint: 'search',
      },
    };

    const res = http.get(
      `${BASE_URL}/get?category=electronics&productId=${customer.productId}`,
      params
    );

    const success = check(res, {
      'search status is 200': (r) => r.status === 200,
      'search response received': (r) => r.body.length > 0,
    });

    failedRequests.add(!success);
  });

  sleep(1);

  // --------------------------------------------------
  // Add product to cart
  // --------------------------------------------------

  group('04 - Add To Cart', function () {
    const payload = JSON.stringify({
      customerId: customer.customerId,
      productId: customer.productId,
      quantity: customer.quantity,
    });

    const res = http.post(
      `${BASE_URL}/anything/cart`,
      payload,
      {
        headers: {
          'Content-Type': 'application/json',
        },
        tags: {
          endpoint: 'cart',
        },
      }
    );

    cartRequests.add(1);

    const success = check(res, {
      'cart request status is 200': (r) => r.status === 200,
      'cart request accepted': (r) => r.json('json.customerId') === customer.customerId,
    });

    failedRequests.add(!success);
  });

  sleep(Math.random() * 2 + 1);

  // --------------------------------------------------
  // Checkout
  // --------------------------------------------------

  group('05 - Checkout', function () {
    const start = Date.now();

    const checkoutPayload = JSON.stringify({
      orderId: `order-${__VU}-${__ITER}`,
      customerId: customer.customerId,
      productId: customer.productId,
      quantity: customer.quantity,
      paymentMethod: 'test-payment',
    });

    const res = http.post(
      `${BASE_URL}/anything/checkout`,
      checkoutPayload,
      {
        headers: {
          'Content-Type': 'application/json',
        },
        tags: {
          endpoint: 'checkout',
        },
      }
    );

    checkoutDuration.add(Date.now() - start);

    const success = check(res, {
      'checkout status is 200': (r) => r.status === 200,
      'checkout response is JSON': (r) =>
        r.headers['Content-Type']?.includes('application/json'),
      'checkout contains order ID': (r) =>
        r.json('json.orderId') !== undefined,
    });

    failedRequests.add(!success);
  });

  sleep(Math.random() * 3 + 1);
}

// ----------------------------------------------------
// Background API health monitoring
// ----------------------------------------------------

export function apiHealthCheck() {
  group('API Health Check', function () {
    const res = http.get(`${BASE_URL}/status/200`, {
      tags: {
        endpoint: 'health',
      },
    });

    check(res, {
      'health endpoint is 200': (r) => r.status === 200,
    });
  });

  sleep(5);
}