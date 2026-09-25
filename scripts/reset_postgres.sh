#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
docker-compose exec -T postgres psql -U fnx -d fnx < scripts/reset_postgres.sql
echo "Postgres app data cleared (tests, tests_runs, jobs)."
