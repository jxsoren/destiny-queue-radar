# destiny-queue-radar

See when Destiny 2 players on your watchlist are in a PvP match, so you can time your queue.

> **Status:** early development. The backend (API, live stream, sign-in, poller) and the dashboard are in place.

## Requirements

- Go 1.25+
- Node 22+ and npm (for the dashboard)
- A Bungie.net API key ([register an application](https://www.bungie.net/en/Application))

## Getting started

### Probe (records raw Bungie responses)

```bash
export BUNGIE_API_KEY=your-key
go run ./cmd/probe -players "Name#1234"
```

### Server

1. Create a database: `createdb destiny_queue_radar`
2. Create a trusted local certificate (one time):
   ```bash
   brew install mkcert && mkcert -install
   mkdir -p certs && mkcert -cert-file certs/localhost.pem -key-file certs/localhost-key.pem localhost 127.0.0.1
   ```
3. In your Bungie application settings, set the OAuth client type to **Confidential** and the redirect URL to `https://localhost:8443/auth/callback`.
4. Put the settings below in `.env`, then run:
   ```bash
   set -a; . ./.env; set +a
   go run ./cmd/server
   ```
5. Open https://localhost:8443 and sign in (run `make build` first to embed the dashboard).

## Configuration

| Variable | Required | Default |
|---|---|---|
| `BUNGIE_API_KEY` | yes | none |
| `BUNGIE_BASE_URL` | no | `https://www.bungie.net` |
| `DATABASE_URL` | server | none |
| `BUNGIE_OAUTH_CLIENT_ID` | server | none |
| `BUNGIE_OAUTH_CLIENT_SECRET` | server | none |
| `BUNGIE_OAUTH_REDIRECT_URL` | server | none |
| `TOKEN_ENCRYPTION_KEY` | server | none (`openssl rand -base64 32`) |
| `BUNGIE_STATS_BASE_URL` | no | `https://stats.bungie.net` |
| `ADDR` | no | `:$PORT` if `PORT` is set, else `:8443` |
| `TLS_CERT_FILE` / `TLS_KEY_FILE` | no | none (plain HTTP) |
| `LOG_FORMAT` | no | `text` (`json` in the Docker image) |
| `SENTRY_DSN` | no | none (Sentry off) |
| `SENTRY_ENVIRONMENT` | no | `RAILWAY_ENVIRONMENT_NAME` |
| `SENTRY_RELEASE` | no | `RAILWAY_GIT_COMMIT_SHA` |

In those rows, `server` means "required for cmd/server only".

## Deploying to Railway

The `Dockerfile` builds the dashboard and server into one image, and `railway.json` tells Railway to build it and gate each deploy on `/readyz`.

1. Create a Railway project from this repo and add a **PostgreSQL** service.
2. On the app service, set the variables: `DATABASE_URL=${{Postgres.DATABASE_URL}}`, `BUNGIE_API_KEY`, `BUNGIE_OAUTH_CLIENT_ID`, `BUNGIE_OAUTH_CLIENT_SECRET`, `TOKEN_ENCRYPTION_KEY`, and `BUNGIE_OAUTH_REDIRECT_URL=https://<your-domain>/auth/callback`. Optionally set `SENTRY_DSN` to report errors. Leave `ADDR` and the TLS settings unset: Railway sets `PORT` and terminates HTTPS.
3. Generate a domain under the service's networking settings, and add the same callback URL to your Bungie application.
4. Keep one replica. The live stream hub is in memory, so a second instance would miss events.

With `SENTRY_DSN` set, every Error-level log line and every recovered panic becomes a Sentry event, grouped by log message and limited to one event per message per minute. The logs keep every line.

Health endpoints:

- `GET /healthz`: liveness; 200 while the process serves HTTP.
- `GET /readyz`: readiness; 503 if the database ping fails or the poller has not looped in 30 seconds. It reports Bungie's state but does not fail on a Bungie outage. Point an uptime monitor here.

## Development

```bash
go test ./...
go vet ./...
gofmt -l .
```

### Dashboard (`web/`)

```bash
cd web
npm ci
npm run dev        # http://localhost:5173, proxies /api and /auth to https://localhost:8443
npm run typecheck
npm test
npm run build      # writes internal/webui/dist/app, embedded by the next go build
```

Sign in through the Go server (https://localhost:8443), then open http://localhost:5173: the session cookie is host-only for `localhost`, so the dev server sends it too.

`make build` builds the dashboard and then `bin/server` with it embedded. Without a dashboard build, the server serves a placeholder page.

### End-to-end test

Runs the dashboard against `cmd/fakebungie` and an e2e build of the server (`-tags e2e`, which adds a test-only sign-in at `/auth/e2e-login` and polls every 2 seconds; never deploy it).

```bash
createdb -h /tmp destiny_queue_radar_e2e   # once; the test wipes this database's public schema
cd web
npx playwright install chromium            # once
E2E_DATABASE_URL="postgres:///destiny_queue_radar_e2e?host=/tmp" npm run e2e
```

`go run ./cmd/fakebungie` on its own lets you run the dashboard without a Bungie API key; change a player with `curl -X POST -d '{"state":"match"}' http://127.0.0.1:9100/fake/players/4611686018400000002/state`.

## Project layout

```
cmd/probe/          CLI that polls player profiles and records responses
cmd/server/         HTTP server: API, live stream, sign-in, poller
internal/auth/      Bungie OAuth tokens, sessions, cookies
internal/bungie/    Bungie.net API client
internal/classifier/ Profile → player status rules
internal/config/    Environment configuration
internal/cryptobox/ Token encryption at rest
internal/engine/    Polling scheduler and worker pool
internal/hub/       Live event fan-out
internal/manifest/  Activity and mode definitions
internal/matches/   Recent PvP matches and lobbies
internal/probe/     Probe summaries and recording
internal/ratelimit/ Global Bungie request pacing
internal/status/    Player status types
internal/store/     Postgres storage and migrations
internal/web/       HTTP handlers
internal/webui/     Embedded dashboard build (placeholder when unbuilt)
web/                React dashboard source
```

## Disclaimer

Not affiliated with or endorsed by Bungie. Destiny is a trademark of Bungie, Inc.
