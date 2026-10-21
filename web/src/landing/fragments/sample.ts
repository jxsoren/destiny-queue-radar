import type { Match, Participant, Player, Status } from '../../api/types';
import { buildGroups, type PlayerRow } from '../../palette/rows';
import { demoBanner, demoEmblem } from '../demo/script';

// Made-up players for the landing page's fragments. The page labels them as made up and never links them.

/** Emblems for the lobby players who are not on the demo board, drawn in the same flat language. */
const EXTRA_MARKS: Record<string, [ground: string, mark: string]> = {
  wren: ['#1d1d1d', '<path d="M8 11 H28 M8 18 H28 M8 25 H28" stroke="#9b9b9b" stroke-width="3"/>'],
  pike: ['#2c2c2c', '<path d="M8 24 L18 12 L28 24" fill="none" stroke="#eaeaea" stroke-width="4"/>'],
  sable: ['#3a3a3a', '<circle cx="18" cy="18" r="10" fill="none" stroke="#eaeaea" stroke-width="3"/><circle cx="18" cy="18" r="3" fill="#eaeaea"/>'],
  ash: ['#1d1d1d', '<path d="M8 28 A10 10 0 0 1 28 28 Z" fill="#9b9b9b"/><rect x="8" y="8" width="20" height="4" fill="#eaeaea"/>'],
};

export function drawnEmblem(key: keyof typeof EXTRA_MARKS): string {
  const [ground, mark] = EXTRA_MARKS[key];
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 36 36"><rect width="36" height="36" fill="${ground}"/>${mark}</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

const participant = (membershipId: string, bungieName: string, emblemPath: string, team: number, isSelf = false): Participant => ({
  membershipType: 3,
  membershipId,
  bungieName,
  emblemPath,
  team,
  watched: false,
  isSelf,
});

/** The visitor's last match: Trials on Endless Vale, three against three. Rook and Mire match the demo board. */
export const LAST_MATCH: Match = {
  instanceId: 'sample-last-match',
  period: '2026-10-16T19:48:00Z',
  activity: 'Endless Vale',
  playlist: 'Trials',
  players: [
    participant('sample-wren', 'Wren#4420', drawnEmblem('wren'), 0, true),
    participant('sample-pike', 'Pike#0937', drawnEmblem('pike'), 0),
    participant('sample-sable', 'Sable#6624', drawnEmblem('sable'), 0),
    participant('demo-rook', 'Rook#2093', demoEmblem(0), 1),
    participant('demo-mire', 'Mire#3307', demoEmblem(1), 1),
    participant('sample-ash', 'Ash#5150', drawnEmblem('ash'), 1),
  ],
};

export const LOBBY_LABEL = 'YOUR LAST MATCH · TRIALS · ENDLESS VALE · 6 PLAYERS';
export const ROOK_ID = 'demo-rook';

/** The palette's "Your last match" rows, built by the palette's own row builder. */
export function lobbyRows(query: string, watched: ReadonlySet<string>): PlayerRow[] {
  const groups = buildGroups({ matches: [LAST_MATCH], expanded: new Set(), query, search: null, watched, now: 0 });
  return groups.last.filter((r): r is PlayerRow => r.kind === 'player');
}

/** A fixed clock for the static fragments, so their timers read the same on every visit. */
export const SAMPLE_NOW = Date.parse('2026-10-16T20:00:00Z');
const ago = (ms: number) => new Date(SAMPLE_NOW - ms).toISOString();

function samplePlayer(membershipId: string, bungieName: string, seed: number, status: Status, polled = true): Player {
  return {
    membershipType: 3,
    membershipId,
    bungieName,
    emblemPath: demoEmblem(seed),
    emblemBackgroundPath: demoBanner(seed),
    status,
    lastPolledAt: polled ? ago(5_000) : null,
    lastSuccessAt: polled ? ago(5_000) : null,
    source: 'match',
    addedAt: ago(polled ? 86_400_000 : 2_000),
  };
}

const rook = (status: Status, polled: boolean) => samplePlayer(ROOK_ID, 'Rook#2093', 0, status, polled);

/** Just added from the palette: the first check has not finished. */
export const ROOK_CHECKING = rook({ state: '' }, false);
/** Nine minutes and 48 seconds into Trials on Burnout. */
export const ROOK_IN_MATCH = rook({ state: 'IN_MATCH', playlist: 'Trials', activity: 'Burnout', startedAt: ago(588_000) }, true);
/** A minute after that match ended, back in orbit. */
export const ROOK_QUEUING = rook(
  { state: 'LIKELY_QUEUING', place: 'ORBIT', lastPvpPlaylist: 'Trials', lastPvpEndedAt: ago(60_000) },
  true,
);

/** The first screen's board preview (spec §5.1): two in a match, one queuing. */
export const PREVIEW_IN_MATCH: readonly Player[] = [
  ROOK_IN_MATCH,
  samplePlayer('demo-mire', 'Mire#3307', 1, { state: 'IN_MATCH', playlist: 'Competitive', activity: 'Javelin-4', startedAt: ago(107_000) }),
];
export const PREVIEW_QUEUING: readonly Player[] = [
  samplePlayer('demo-kestrel', 'Kestrel#0412', 2, { state: 'LIKELY_QUEUING', place: 'ORBIT', lastPvpPlaylist: 'Control', lastPvpEndedAt: ago(120_000) }),
];
