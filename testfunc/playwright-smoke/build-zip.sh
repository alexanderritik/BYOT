#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
if [[ ! -d node_modules ]]; then
  npm install
fi
rm -f ../playwright-smoke.zip
zip -r ../playwright-smoke.zip . \
  -x '*.git*' \
  -x 'build-zip.sh' \
  -x '.gitignore'
echo "Wrote $(dirname "$PWD")/playwright-smoke.zip"
