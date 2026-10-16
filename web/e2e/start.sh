#!/usr/bin/env bash
# Builds and starts the fake Bungie API and an e2e build of the server for Playwright.
# Requires E2E_DATABASE_URL; its public schema is dropped and recreated on every run.
set -euo pipefail
cd "$(dirname "$0")/../.."

: "${E2E_DATABASE_URL:?set E2E_DATABASE_URL, e.g. postgres:///destiny_queue_radar_e2e?host=/tmp}"

tmp="$(mktemp -d)"
cleanup() {
  kill $(jobs -p) 2>/dev/null || true
  rm -rf "$tmp"
}
trap cleanup EXIT INT TERM

psql "$E2E_DATABASE_URL" -v ON_ERROR_STOP=1 -q -c 'DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;'
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj '/CN=localhost' \
  -keyout "$tmp/key.pem" -out "$tmp/cert.pem" 2>/dev/null

npm --prefix web run build
go build -o "$tmp/fakebungie" ./cmd/fakebungie
go build -tags e2e -o "$tmp/server" ./cmd/server

"$tmp/fakebungie" -addr 127.0.0.1:9100 &

export BUNGIE_API_KEY=fake
export BUNGIE_BASE_URL=http://127.0.0.1:9100
export BUNGIE_STATS_BASE_URL=http://127.0.0.1:9100
export DATABASE_URL="$E2E_DATABASE_URL"
export BUNGIE_OAUTH_CLIENT_ID=1
export BUNGIE_OAUTH_CLIENT_SECRET=fake
export BUNGIE_OAUTH_REDIRECT_URL=https://localhost:9443/auth/callback
export TOKEN_ENCRYPTION_KEY="$(openssl rand -base64 32)"
export ADDR=:9443
export TLS_CERT_FILE="$tmp/cert.pem"
export TLS_KEY_FILE="$tmp/key.pem"
"$tmp/server" &

wait
