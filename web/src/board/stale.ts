import type { Player } from '../api/types';
import { since } from '../lib/format';

export const STALE_AFTER_MS = 3 * 60_000;

/**
 * How long a player's status has been stale, or null while it is fresh (spec §3.3).
 * A player never polled is still being checked, not stale. One polled but never successfully
 * is stale since it was added.
 */
export function staleFor(p: Player, now: number): number | null {
  if (p.lastSuccessAt === null) {
    if (p.lastPolledAt === null) return null;
    return since(p.addedAt, now);
  }
  const age = since(p.lastSuccessAt, now);
  return age !== null && age > STALE_AFTER_MS ? age : null;
}

/** The newest successful poll across players (for "UPDATED 23S AGO"), or null. */
export function latestSuccess(players: Iterable<Player>): number | null {
  let latest: number | null = null;
  for (const p of players) {
    if (!p.lastSuccessAt) continue;
    const t = Date.parse(p.lastSuccessAt);
    if (!Number.isNaN(t) && (latest === null || t > latest)) latest = t;
  }
  return latest;
}
