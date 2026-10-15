# destiny-queue-radar

See when Destiny 2 players on your watchlist are in a PvP match, so you can time your queue.

> **Status:** early development. The Bungie API client and a probe tool are in place; the web app is not built yet.

## Requirements

- Go 1.25+
- A Bungie.net API key ([register an application](https://www.bungie.net/en/Application))

## Getting started

```bash
export BUNGIE_API_KEY=your-key
go run ./cmd/probe -players "Name#1234"
```

## Configuration

| Variable | Required | Default |
|---|---|---|
| `BUNGIE_API_KEY` | yes | none |
| `BUNGIE_BASE_URL` | no | `https://www.bungie.net` |

## Development

```bash
go test ./...
go vet ./...
gofmt -l .
```

## Project layout

```
cmd/probe/         CLI that polls player profiles and records responses
internal/bungie/   Bungie.net API client
internal/manifest/ Activity and mode definitions
internal/probe/    Probe summaries and recording
internal/config/   Environment configuration
```

## Disclaimer

Not affiliated with or endorsed by Bungie. Destiny is a trademark of Bungie, Inc.
