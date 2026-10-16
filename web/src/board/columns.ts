import type { Player, State } from '../api/types';

export type ColumnId = 'match' | 'queue' | 'idle' | 'offline';

export interface ColumnDef {
  id: ColumnId;
  /** Column heading. */
  title: string;
  /** Counts bar label (spec §3.2). */
  label: string;
}

export const COLUMNS: readonly ColumnDef[] = [
  { id: 'match', title: 'IN MATCH', label: 'IN MATCH' },
  { id: 'queue', title: 'LIKELY QUEUING', label: 'QUEUING?' },
  { id: 'idle', title: 'NOT IN PVP', label: 'NOT IN PVP' },
  { id: 'offline', title: 'OFFLINE', label: 'OFFLINE' },
];

export function columnOf(state: State): ColumnId {
  switch (state) {
    case 'IN_MATCH':
      return 'match';
    case 'LIKELY_QUEUING':
      return 'queue';
    case 'OFFLINE':
    case 'PRIVATE':
    case 'NOT_FOUND':
      return 'offline';
    default:
      return 'idle'; // NOT_IN_PVP, and "" before the first poll
  }
}

const time = (isoTime: string | undefined): number => (isoTime ? Date.parse(isoTime) : Number.NaN);

function nameKey(p: Player): string {
  return p.bungieName ? p.bungieName.toLowerCase() : `￿${p.membershipId}`;
}

function byName(a: Player, b: Player): number {
  const ak = nameKey(a);
  const bk = nameKey(b);
  if (ak !== bk) return ak < bk ? -1 : 1;
  return a.membershipId < b.membershipId ? -1 : a.membershipId > b.membershipId ? 1 : 0;
}

/** Orders by a time, missing times last; ties by name. desc puts the latest first. */
function byTime(get: (p: Player) => number, desc: boolean) {
  return (a: Player, b: Player): number => {
    const ta = get(a);
    const tb = get(b);
    const na = Number.isNaN(ta);
    const nb = Number.isNaN(tb);
    if (na || nb) return na === nb ? byName(a, b) : na ? 1 : -1;
    if (ta !== tb) return desc ? tb - ta : ta - tb;
    return byName(a, b);
  };
}

const SORTS: Record<ColumnId, (a: Player, b: Player) => number> = {
  match: byTime((p) => time(p.status.startedAt), false), // earliest start = longest in match
  queue: byTime((p) => time(p.status.lastPvpEndedAt), true),
  idle: byName,
  offline: byName,
};

/** Splits players into the four columns, each sorted per spec §3.3. */
export function groupPlayers(players: Iterable<Player>): Record<ColumnId, Player[]> {
  const out: Record<ColumnId, Player[]> = { match: [], queue: [], idle: [], offline: [] };
  for (const p of players) out[columnOf(p.status.state)].push(p);
  for (const id of Object.keys(out) as ColumnId[]) out[id].sort(SORTS[id]);
  return out;
}
