#!/usr/bin/env bash
# Builds playwright-todo.zip for upload (runtime=playwright).
# The bundle must include node_modules, and @playwright/test must stay at
# 1.49.1 to match BYOT's Playwright image.
set -euo pipefail
cd "$(dirname "$0")"
npm install --no-audit --no-fund
rm -f ../playwright-todo.zip
zip -qr ../playwright-todo.zip . -x 'build-zip.sh' -x '.gitignore' -x 'test-results/*'
echo "Wrote $(cd .. && pwd)/playwright-todo.zip"
