# destiny-queue-radar

See when Destiny 2 players on your watchlist are in a PvP match, so you can time your queue.

> **Status:** early development. The Bungie API client, probe tool and backend server (API, live stream, sign-in, poller) are in place; the frontend is not built yet.

## Requirements

- Go 1.25+
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
5. Open https://localhost:8443 and sign in.

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
| `ADDR` | no | `:8443` |
| `TLS_CERT_FILE` / `TLS_KEY_FILE` | no | none (plain HTTP) |

In those rows, `server` means "required for cmd/server only".

## Development

```bash
go test ./...
go vet ./...
gofmt -l .
```

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
```

## Disclaimer

Not affiliated with or endorsed by Bungie. Destiny is a trademark of Bungie, Inc.
