# Probe Runbook (Step 0)

Goal: confirm how Bungie's API reports each player state before writing the classifier (spec §5.5).

## Setup

1. Register an app at https://www.bungie.net/en/Application (OAuth type can be "Not applicable" for the probe). The API key is shown on the app page after creation. The Origin header and OAuth settings are not needed for the probe.
2. `export BUNGIE_API_KEY=...`
3. Get 1–3 consenting players (you plus friends) with public profiles, and ideally one with a **private** profile.

Note: the first run downloads the manifest (tens of MB), which can take a while on slow connections. If it times out, just rerun.

## Run

```bash
mkdir -p testdata/probe
go run ./cmd/probe -players "You#1234,Friend#5678,PrivateFriend#0001" -interval 15s 2>&1 | tee -a testdata/probe/session.log
```

Stdin stays interactive, so you can still type labels. Label set-times, `took=` values, and `throttled=`/`error` lines appear only in `session.log`, not in `index.jsonl`.

At startup the probe prints the player mapping (`P1 = You#1234`, `P2 = ...`). Use it to refer to players in labels.

Before each scenario, type its label into the probe terminal and press Enter. The label is logged with a timestamp (`label → "..." (set at HH:MM:SS.mmm)`).

**One label applies to ALL players.** So keep every player in the same state for a scenario and use player-neutral labels (`all orbit`, `all in comp`). When players must differ, put the P number in the label (`P1 in comp, P2 orbit`) and filter `index.jsonl` by both player and label.

Hold each state for at least 3 polls so changes and steady state are distinguishable.

## Scenarios (label, what to do, hold for)

| Label | Action | Hold for |
|---|---|---|
| `all offline` | Fully signed out of D2 | 1 min |
| `all orbit` | Sitting in Orbit | 1 min |
| `all orbit queuing comp` | In Orbit, matchmaking for Competitive | Until match found |
| `all loading comp` | Loading into the match | Until in match |
| `all in comp` | In a Competitive match | Whole match |
| `MATCH ENDED` | Type it the instant the end-of-match screen appears | See below |
| `all post-game` | Post-game screen / back in Orbit | 2 min |
| `all tower` | In the Tower | 1 min |
| `all pve` | Patrol or strike | 1 min |
| `all trials` | One Trials match (if live) | Whole match |
| `P1 P2 fireteam orbit` / `P1 P2 fireteam in comp` | Repeat `orbit` and `in comp` in a fireteam | 1 min / whole match |
| `P3 private` | Private player doing anything | 1 min |
| `all offline again` | Sign out after everything else, to capture the just-signed-out state | 1 min |

### Match-end scenarios

Start the whole Comp block (`in comp` → `MATCH ENDED` → `post-game`) at `-interval 5s` for finer resolution. Don't restart mid-session, because each restart re-downloads the manifest. Lag = the first poll where the activity changes minus the logged `MATCH ENDED` label time.

### Optional burst step

Run `-interval 1s` for 60 seconds with all players to see whether throttling ever appears. Normal use (~0.2 req/s) is unlikely to trigger it.

## Questions to answer (write the answers in Findings)

1. Can Orbit and social spaces (Tower etc.) be told apart from PvE? Which fields: activity hash/name, mode hash/category, `isPvP`?
2. When a match ends, how many polls until the activity changes? Is there a post-game state?
3. Does anything change between "in Orbit" and "in Orbit, queuing"? (Expected: no.)
4. What does an offline player look like (transitory data missing? stale activity hash?)
5. What does a private player look like (which components have `privacy: 2`, which have data)? Note: a private player shows `online=false` even when online. Check `priv=...trans:private` to tell the two apart.
6. Is `currentActivityModeHash` or `currentPlaylistActivityHash` the better source for the playlist (Comp vs Control vs Trials)?
7. Did we see any throttling at this request rate? The client retries throttles silently, so the evidence is `took=` values over ~2s or a logged `throttled=true`.
8. With multiple characters or a fireteam, does `ActiveCharacterID` pick the character being played, and does party size show up in transitory data? Include the step: P1 switches character in Orbit; confirm `class=` follows.

## Findings

Write the findings to `docs/superpowers/specs/2026-10-15-probe-findings.md` with one section per question above, quoting evidence. Lag and throttle evidence comes from `session.log`; state evidence comes from `index.jsonl`. Plan 2's classifier is designed from that document.

## Fixtures for Plan 2

After the session, copy one representative raw JSON per state (offline, private, orbit, tower, pve, in-comp, post-game) from `testdata/probe/` into `testdata/bungie/` (committed). Get each player's consent first, and anonymize before committing: replace `membershipId`, `bungieGlobalDisplayName` and `bungieGlobalDisplayNameCode`, `characterIds`, the character-ID map keys under `characters` and `characterActivities`, and `partyMembers[].membershipId`/`displayName`. Character IDs must be replaced consistently everywhere.
