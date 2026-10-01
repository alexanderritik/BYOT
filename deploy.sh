#!/usr/bin/env bash
set -euo pipefail

HOST="${HOST:-opc@161.118.160.217}"
KEY="${KEY:-$HOME/.ssh/oracle_mini_lambda.key}"
SSH="ssh -i $KEY"

cd "$(dirname "$0")"

echo "==> building linux/amd64"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /tmp/mini-lambda-linux .

echo "==> uploading"
scp -i "$KEY" /tmp/mini-lambda-linux "$HOST:/tmp/mini-lambda"
scp -i "$KEY" .env "$HOST:/opt/mini-lambda/.env"

echo "==> restarting"
$SSH "$HOST" '
  chmod 600 /opt/mini-lambda/.env
  mv /tmp/mini-lambda /opt/mini-lambda/mini-lambda
  chmod +x /opt/mini-lambda/mini-lambda
  sudo chcon -t bin_t /opt/mini-lambda/mini-lambda
  sudo systemctl restart mini-lambda
  sleep 2
  sudo systemctl --no-pager status mini-lambda | head -5
  curl -sf localhost:3000/health && echo
'
