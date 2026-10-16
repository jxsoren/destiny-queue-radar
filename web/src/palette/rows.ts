import type { Match, Participant, SearchResult, Source } from '../api/types';
import { formatAgo, since } from '../lib/format';

export interface PlayerRow {
  kind: 'player';
  key: string;
  membershipType: number;
  membershipId: string;
  bungieName: string;
  emblemPath: string;
  isSelf: boolean;
  watching: boolean;
  source: Source;
}

export interface LobbyRow {
  kind: 'lobby';
  key: string;
  instanceId: string;
  summary: string;
  expanded: boolean;
}

export type Row = PlayerRow | LobbyRow;

export interface PaletteGroups {
  /** 1. Your last match: every participant. */
  last: Row[];
  /** 2. Older lobbies: matches 2-10, one row each, expandable. */
  older: Row[];
  /** 3. Bungie search. */
  search: Row[];
}

export interface BuildInput {
  matches: readonly Match[];
  expanded: ReadonlySet<string>;
  query: string;
  search: SearchResult | null;
  /** Players on the watchlist, including ones added in this palette session. */
  watched: ReadonlySet<string>;
  now: number;
}

const OLDER_LOBBIES = 9;

export function isBungieName(q: string): boolean {
  return /^.+#\d{4}$/.test(q.trim());
}

/** "COMPETITIVE · JAVELIN-4 · 6 PLAYERS · 41M"; the age counts from the match start. */
export function lobbySummary(m: Match, now: number): string {
  const ago = since(m.period, now);
  return [m.playlist, m.activity, `${m.players.length} players`, ago === null ? '' : formatAgo(ago)]
    .filter(Boolean)
    .map((s) => s.toUpperCase())
    .join(' · ');
}

export function buildGroups({ matches, expanded, query, search, watched, now }: BuildInput): PaletteGroups {
  const q = query.trim().toLowerCase();
  const hit = (p: Participant) => q === '' || p.bungieName.toLowerCase().includes(q);
  const toRow = (m: Match, p: Participant): PlayerRow => ({
    kind: 'player',
    key: `${m.instanceId}:${p.membershipId}`,
    membershipType: p.membershipType,
    membershipId: p.membershipId,
    bungieName: p.bungieName,
    emblemPath: p.emblemPath,
    isSelf: p.isSelf,
    watching: p.watched || watched.has(p.membershipId),
    source: 'match',
  });

  const [first, ...rest] = matches;
  const last: Row[] = first ? first.players.filter(hit).map((p) => toRow(first, p)) : [];

  const older: Row[] = [];
  for (const m of rest.slice(0, OLDER_LOBBIES)) {
    const hits = m.players.filter(hit);
    if (q !== '' && hits.length === 0) continue;
    const open = q !== '' || expanded.has(m.instanceId);
    older.push({ kind: 'lobby', key: m.instanceId, instanceId: m.instanceId, summary: lobbySummary(m, now), expanded: open });
    if (open) older.push(...hits.map((p) => toRow(m, p)));
  }

  const searchRows: Row[] = search
    ? [
        {
          kind: 'player',
          key: `search:${search.membershipId}`,
          membershipType: search.membershipType,
          membershipId: search.membershipId,
          bungieName: search.bungieName,
          emblemPath: search.emblemPath,
          isSelf: false,
          watching: watched.has(search.membershipId),
          source: 'search',
        },
      ]
    : [];

  return { last, older, search: searchRows };
}

/** The single keyboard list across the three groups. */
export function flatten(g: PaletteGroups): Row[] {
  return [...g.last, ...g.older, ...g.search];
}
