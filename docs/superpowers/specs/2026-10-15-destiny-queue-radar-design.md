# Destiny Queue Radar — Design Spec

**Date:** 2026-10-15
**Status:** Draft, awaiting review

## 1. Purpose

Destiny Queue Radar is a public web app that lets Destiny 2 players privately watch other players (typically suspected cheaters) and see, live, whether those players are in a PvP match or likely re-queuing. Users can then time their own queue to avoid matching against them.

### Goals (v1)

- A live dashboard ("board") showing every watched player's current status.
- Confirmed **in-match** detection for PvP playlists (Competitive, Trials of Osiris, Iron Banner, Control, etc.).
- An inferred, clearly labeled **likely queuing** state.
- Add players by Bungie Name search or by picking them from your own recent PvP matches.
- Automatic polling (~60s) plus a rate-limited manual refresh.

### Non-goals (v1)

- Detecting actual matchmaking/queue state. The Bungie API does not expose this; "likely queuing" is an inference.
- Notifications (browser push, Discord). Dashboard only.
- Public, shared, or community watchlists, or any public "flagged" counts. Watchlists are strictly private.
- Inferring a private player's status via their fireteam or teammates. Private profiles show as Private.
- Per-user playlist filters. All PvP counts as a threat; all PvE is treated as safe.
- Player stats (K/D, Elo, flawless counts, etc.).
- Running more than one server instance.

## 2. Users and Access

- Public site; anyone can sign up.
- Sign-in is **Bungie OAuth only**. No anonymous use.
- Each user has one private watchlist, capped at **50 players**.
- Watchlists are never visible to other users, and watched players are never contacted or listed anywhere.

## 3. User Experience

### 3.1 Board layout (vertical status columns)

Four columns, left to right:

| Column | Contains |
|---|---|
| **In match** (red) | `IN_MATCH` |
| **Likely queuing** (amber) | `LIKELY_QUEUING` |
| **Not in PvP** (blue) | `NOT_IN_PVP` (Orbit, social spaces, PvE) |
| **Offline** (grey) | `OFFLINE`, `PRIVATE`, `NOT_FOUND` |

Board header: watchlist count, "Updated Ns ago", and a **Refresh** button showing its cooldown.

### 3.2 Player card

- **Emblem banner** at the top: the `emblemBackgroundPath` (wide banner) of the player's most recently played character, with the emblem icon on the left and the Bungie Name (name plus `#code`) overlaid, similar to Trials Report.
- A colored left accent that matches the status column.
- A meta line under the banner, depending on state:
  - In match: `Trials · Burnout` + elapsed timer (`04:12`)
  - Likely queuing: `Orbit · Comp ended` + `2m ago`
  - Not in PvP: `Orbit`, `Tower`, or `Busy (PvE) · Raid: Salvation's Edge`, plus `Last: Trials 1h`
  - Offline: `Offline` + `Last PvP: 2d ago`
  - Private: 🔒 `Profile private` + a **Find in your matches** link
- Offline and private cards are dimmed and desaturated.
- A **stale · Nm** marker appears if the player has not been polled successfully in 3 minutes.
- The user's optional note for that player is shown on the card.

### 3.3 Adding players

- **Search:** enter a Bungie Name (`Name#1234`) and click Watch.
- **Recent matches picker:** shows the user's last 10 PvP matches with each full lobby; click Watch next to any participant.
- **Find in your matches** (private cards): opens the picker filtered to matches the user shared with that player, so the user can manually watch that player's teammates.

### 3.4 Global banner

When the Bungie API is down or degraded, a banner across the board reads "Bungie API unavailable — statuses frozen" (or "degraded").

## 4. Architecture

Tech stack:

- **Backend:** Go, standard-library `net/http` routing, `pgx` + `sqlc`, Postgres.
- **Frontend:** React + Vite + TypeScript, Tailwind + shadcn/ui, TanStack Query, native `EventSource`.
- **Deployment:** one container serving the Go API and the built SPA as static files, plus managed Postgres (Fly.io / Railway / VPS).

### 4.1 Go packages

| Package | Responsibility |
|---|---|
| `auth` | Bungie OAuth login/callback, sessions, token refresh |
| `api` | REST handlers: watchlist CRUD, search, recent matches, manual refresh |
| `hub` | SSE connections; tracks online users; fans out status changes only to viewers watching that player |
| `scheduler` | Computes the active set; schedules polls; priority queue |
| `poller` | Worker pool behind a global rate limiter; fetches profiles |
| `classifier` | **Pure function**: `(profile, previous state, now) → new state`. No I/O |
| `manifest` | In-memory activity/mode definitions; reloads when Bungie's manifest version changes |
| `bungie` | Typed Bungie API client (ported from Guardian Shuffle): retries, backoff, error-code handling |
| `store` | sqlc-generated queries over pgx |

### 4.2 Poll cycle

1. The scheduler picks the next due player from the active set.
2. The poller fetches the profile with components **100, 200, 204, 1000**.
3. The classifier computes the new status.
4. If the status or detail changed, the `players` row is updated and the hub pushes `player_update` to every connected viewer who watches that player.

### 4.3 Active set

The active set is every player on the watchlist of at least one user who has an open SSE stream, or who closed it less than **5 minutes** ago. Each player is polled **once per interval**, no matter how many users watch them. Players outside the active set are not polled.

### 4.4 Polling cadence

- Routine interval: **60s** per active player.
- Manual refresh: `POST /api/refresh` enqueues the user's watched players at top priority. Limited to **one per 30s per user**. Players polled in the last 10s are skipped.

## 5. Status Classification

### 5.1 Inputs

- Components 100 (profile), 200 (characters; emblem), 204 (current activities per character), 1000 (live session data).
- The **active character** is the one with the most recent `dateActivityStarted`.
- Stored per player: previous `status`, `last_pvp_ended_at`, `last_pvp_playlist`.

### 5.2 States, evaluated in order

| # | State | Rule |
|---|---|---|
| 1 | `PRIVATE` | Bungie returns a privacy-restricted response for the needed components |
| 2 | `OFFLINE` | No live session data (component 1000), and no character has a current activity |
| 3 | `IN_MATCH` | The current activity's mode category is PvP |
| 4 | `LIKELY_QUEUING` | Not in PvP **and** in Orbit or a social space **and** `last_pvp_ended_at` < **5 min** ago |
| 5 | `NOT_IN_PVP` | Everything else. Sub-label: `ORBIT`, `SOCIAL`, or `PVE` (with activity name) |

`NOT_FOUND` is an additional terminal state used when the Bungie account no longer exists (see §7.2).

### 5.3 Transition rules

- **IN_MATCH → any other state:** set `last_pvp_ended_at = now` and `last_pvp_playlist` to the playlist just left.
- **IN_MATCH → PvE:** goes straight to `NOT_IN_PVP`; never `LIKELY_QUEUING`.
- **LIKELY_QUEUING expiry:** once the 5-minute window passes with no new PvP activity, the next classification yields `NOT_IN_PVP`.
- The 5-minute window is a single config constant.

### 5.4 Cold start

When a player enters the active set and has no recorded `last_pvp_ended_at`, make **one** activity-history call (PvP modes) to fill in the most recent PvP match and when it ended (period + duration). This also fills "Last PvP" on offline cards. Afterwards, history is only refreshed when the player leaves IN_MATCH.

### 5.5 Assumptions to verify (Step 0 API test)

1. Orbit and social spaces can be told apart from PvE using 204/1000 data plus the manifest.
2. The current activity updates promptly when a match ends.
3. The response shape of an offline player and of a private player.

If any assumption fails, only `classifier` (and possibly `poller`'s component list) changes.

## 6. Data Model and API

### 6.1 Postgres tables

```
users          id · bungie_membership_id · bungie_name · created_at
sessions       id (hashed token) · user_id · expires_at
oauth_tokens   user_id · access_token (encrypted) · refresh_token (encrypted) · expires_at

players        membership_id (PK) · membership_type · bungie_name ·
               emblem_path · emblem_background_path ·
               status · status_detail (jsonb: label, activity, mode, started_at) ·
               last_pvp_ended_at · last_pvp_playlist · is_private ·
               last_polled_at · last_success_at

watch_entries  user_id · membership_id · note · source (search | match) · added_at
               PK (user_id, membership_id)
```

- `players` is a shared status cache: one row per player, however many users watch them.
- Match reports (PGCRs) never change once a match is over, so they are cached in an in-memory LRU, not in the database.

### 6.2 REST endpoints

| Method | Path | Purpose |
|---|---|---|
| GET | `/auth/login` | Start Bungie OAuth |
| GET | `/auth/callback` | OAuth callback; create session |
| POST | `/auth/logout` | End session |
| GET | `/api/me` | Current user |
| DELETE | `/api/me` | Delete account, watchlist and tokens |
| GET | `/api/watchlist` | Watched players with current status |
| POST | `/api/watchlist` | Add `{membershipType, membershipId, note?, source}`; 409 when at the 50-player cap |
| PATCH | `/api/watchlist/{membershipId}` | Edit note |
| DELETE | `/api/watchlist/{membershipId}` | Remove |
| GET | `/api/search?q=Name#1234` | Bungie Name lookup |
| GET | `/api/matches/recent` | User's last 10 PvP matches with full lobbies |
| GET | `/api/matches/{instanceId}` | One match's participants |
| POST | `/api/refresh` | Manual refresh; `202`, or `429` + `Retry-After` |

### 6.3 SSE stream: `GET /api/stream`

| Event | Payload | When |
|---|---|---|
| `snapshot` | Full board | On connect and after reconnect |
| `player_update` | `{membershipId, status, statusDetail, lastPvpEndedAt, polledAt}` | On change only |
| `api_status` | `{bungie: "ok" \| "degraded" \| "down"}` | On change |

Opening the stream marks the user as online for the active set. Closing it starts the 5-minute grace period.

## 7. Error Handling, Rate Limits, Abuse Protection

### 7.1 Rate limiting and capacity

- One global rate limiter shared by all workers, starting at **20 req/s**. Step 0 confirms Bungie's actual limit. At 20 req/s and a 60s interval, about 1,200 active players can be polled per minute.
- Priority order: manual refresh, then cold-start history, then routine polls. Within a tier, the player who has gone longest without a poll goes first.
- Over capacity: the interval stretches evenly for everyone. The board shows real "Updated Ns ago" values.
- Bungie throttle responses: honor the requested wait time and pause the limiter.

### 7.2 Bungie errors

| Situation | Handling |
|---|---|
| 5xx / timeout | Up to 3 retries with exponential backoff and jitter; if still failing, the card shows the stale marker |
| Privacy restricted | `PRIVATE` |
| Account not found | `NOT_FOUND`; the card offers Remove |
| API disabled / maintenance | Pause polling, broadcast `api_status: down`, probe every 60s |
| >50% of calls failing within 1 minute | Broadcast `degraded`, slow polling |
| Unknown activity hash | Label "Unknown activity", trigger manifest reload |

### 7.3 Auth and sessions

- OAuth `state` parameter on login.
- Session cookie: `HttpOnly`, `Secure`, `SameSite=Lax`. Mutating requests also require a custom header (CSRF protection).
- Bungie tokens are encrypted at rest and never logged.
- If a token can't be refreshed, recent matches show "Sign in again". The board keeps working, since polling does not use user tokens.

### 7.4 SSE robustness

- Heartbeat comment every 25s.
- The client reconnects automatically through `EventSource`; the server sends `snapshot` on reconnect.
- Each connection has a bounded outgoing buffer. A client that falls behind is dropped and reconnects with a snapshot.

### 7.5 Abuse protection

- Bungie OAuth required. 50 players per watchlist. One manual refresh per 30s per user.
- Search and match lookups: 30 per minute per user.
- Watchlists are private. There is no public flagging or listing of watched players.
- Player rows with no watchers stop being polled immediately and are deleted after 30 days.

## 8. Testing

### 8.1 Step 0: live API test

A Go command (`cmd/probe`) polls a handful of consenting real accounts every 15s while they deliberately move through Orbit, Tower, PvE, a Comp match, match end, offline, and a private profile. It saves every raw response to disk with a label for what was actually happening. Outputs:

- Answers to the assumptions in §5.5.
- Real recorded responses (`testdata/`) used by all later tests.

### 8.2 Backend (TDD, standard `testing` package, table-driven)

| Unit | Coverage |
|---|---|
| `classifier` | Recorded responses → expected states; scripted transitions with an injected clock (IN_MATCH → Orbit at 2m = LIKELY_QUEUING, at 6m = NOT_IN_PVP; IN_MATCH → PvE = NOT_IN_PVP; cold start using history) |
| `bungie` | Fake HTTP server: success, privacy, throttle (wait honored), maintenance, 5xx retries, timeouts |
| `scheduler` | Fake clock and poller: active set and grace period, priority order, fair stretching over capacity, no duplicate in-flight polls |
| `hub` | Updates only reach users watching that player; disconnect cleanup; slow-client drop |
| `store` | Integration tests against real Postgres (Docker): queries, 50-player cap, orphaned player cleanup |
| `api` | Auth required, CSRF header, refresh cooldown (429 + `Retry-After`), watchlist cap, search rate limit |

### 8.3 Fake-Bungie mode

`BUNGIE_BASE_URL` can point at a local fake server that replays recorded responses and scripted scenarios (e.g. "enters Comp → match ends → re-queues"). Used for end-to-end tests, frontend development without an API key, and demos.

### 8.4 Frontend

- **Vitest + Testing Library:** sorting players into columns, the live-update reducer, stale marker, private/offline cards, refresh cooldown state.
- **Playwright (one end-to-end test):** runs against the Go server in fake-Bungie mode. Stubbed login → add a player from recent matches → watch them move In match → Likely queuing → Not in PvP.

### 8.5 CI

GitHub Actions on every PR: `go vet`, `staticcheck`, `go test ./...` with a Postgres service, Vitest, Playwright.

## 9. Future Work (explicitly deferred)

- Notifications (browser push, Discord).
- Per-user playlist filters.
- Player stats on cards (Trials rank, K/D, flawless).
- Multi-instance deployment (Postgres `LISTEN/NOTIFY` or Redis for the hub).
- Any fireteam-based inference for private players. This would require a review of Bungie's API terms first and would be opt-in only.
