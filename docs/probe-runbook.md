# Probe Runbook (Step 0)

Goal: confirm how Bungie's API reports each player state before writing the classifier (spec §5.5).

## Setup

1. Register an app at https://www.bungie.net/en/Application (OAuth type can be "Not applicable" for the probe). Copy the API key.
2. `export BUNGIE_API_KEY=...`
3. Get 1–3 consenting players (you plus friends) with public profiles, and ideally one with a **private** profile.

## Run

```bash
go run ./cmd/probe -players "You#1234,Friend#5678,PrivateFriend#0001" -interval 15s
```

Before each scenario, type its label into the probe terminal and press Enter, e.g. `P1 orbit`.

## Scenarios (label → what to do)

| Label | Action | Hold for |
|---|---|---|
| `P1 offline` | Fully signed out of D2 | 1 min |
| `P1 orbit` | Sitting in Orbit | 1 min |
| `P1 orbit queuing comp` | In Orbit, matchmaking for Competitive | Until match found |
| `P1 loading comp` | Loading into the match | — |
| `P1 in comp` | In a Competitive match | Whole match |
| `P1 post-game` | Post-game screen / back in Orbit | 2 min |
| `P1 tower` | In the Tower | 1 min |
| `P1 pve` | Patrol or strike | 1 min |
| `P1 trials` | One Trials match (if live) | Whole match |
| `P1 fireteam` | Repeat `orbit` and `in comp` in a fireteam with P2 | — |
| `P3 private` | Private player doing anything | 1 min |

## Questions to answer (write the answers in Findings)

1. Can Orbit and social spaces (Tower etc.) be told apart from PvE? Which fields: activity hash/name, mode hash/category, `isPvP`?
2. When a match ends, how many polls until the activity changes? Is there a post-game state?
3. Does anything change between "in Orbit" and "in Orbit, queuing"? (Expected: no.)
4. What does an offline player look like (transitory data missing? stale activity hash?)
5. What does a private player look like (which components have `privacy: 2`, which have data)?
6. Is `currentActivityModeHash` or `currentPlaylistActivityHash` the better source for the playlist (Comp vs Control vs Trials)?
7. Did we see any throttling at this request rate?

## Findings

Write the findings to `docs/superpowers/specs/2026-10-15-probe-findings.md` with one section per question above, quoting `index.jsonl` lines as evidence. Plan 2's classifier is designed from that document.
