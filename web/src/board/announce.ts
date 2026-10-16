import type { Player } from '../api/types';
import { COLUMNS, columnOf, type ColumnId } from './columns';
import { displayName } from './names';

export function columnMap(players: readonly Player[]): Map<string, ColumnId> {
  return new Map(players.map((p) => [p.membershipId, columnOf(p.status.state)]));
}

/** Screen-reader messages for players who moved to another column ("VEX#1111 · IN MATCH"). */
export function columnChanges(prev: ReadonlyMap<string, ColumnId>, players: readonly Player[]): string[] {
  const out: string[] = [];
  for (const p of players) {
    const before = prev.get(p.membershipId);
    const now = columnOf(p.status.state);
    if (before !== undefined && before !== now) {
      const title = COLUMNS.find((c) => c.id === now)?.title ?? now;
      out.push(`${displayName(p).toUpperCase()} · ${title}`);
    }
  }
  return out;
}
