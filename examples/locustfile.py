"""BYOT example: Locust shopper journey against the public Fake Store API.

Upload: runtime=locust, file=locustfile.py
BYOT runs it headless (5 users, up to 30s) and fails the run if any request
fails. Add SLO checks with env vars, e.g. LOCUST_MAX_P95_MS=1500.
Point it at your own service with LOCUST_HOST.

Only load-test services you own or that are published for testing, and keep
the load gentle on shared public APIs like this one.
"""

import random

from locust import HttpUser, between, task

CATEGORIES = ["electronics", "jewelery", "men's clothing", "women's clothing"]


class Shopper(HttpUser):
    host = "https://fakestoreapi.com"
    wait_time = between(1, 3)

    def on_start(self):
        resp = self.client.get("/products?limit=20", name="GET /products")
        self.product_ids = [p["id"] for p in resp.json()] if resp.ok else []

    @task(5)
    def browse_catalog(self):
        self.client.get("/products?limit=20", name="GET /products")

    @task(3)
    def view_product(self):
        if self.product_ids:
            self.client.get(f"/products/{random.choice(self.product_ids)}", name="GET /products/:id")

    @task(2)
    def browse_category(self):
        self.client.get(f"/products/category/{random.choice(CATEGORIES)}", name="GET /products/category/:name")
