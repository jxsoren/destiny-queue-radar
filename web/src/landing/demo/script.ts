import type { Player, PlayerUpdate, Status } from '../../api/types';
import type { BoardAction } from '../../board/reducer';

export const DEMO_STEP_MS = 4_000;

const GREYS = ['#2c2c2c', '#1d1d1d', '#3a3a3a'];
const MARKS = [
  '<circle cx="18" cy="18" r="9" fill="none" stroke="#eaeaea" stroke-width="3"/>',
  '<path d="M8 28 L18 8 L28 28 Z" fill="#eaeaea"/>',
  '<rect x="10" y="10" width="16" height="16" fill="none" stroke="#eaeaea" stroke-width="3"/>',
  '<path d="M18 6 L30 18 L18 30 L6 18 Z" fill="#9b9b9b"/>',
  '<path d="M6 18 H30 M18 6 V30" stroke="#eaeaea" stroke-width="3"/>',
  '<circle cx="18" cy="18" r="5" fill="#ff2a2a"/>',
];

/** A flat emblem drawn in code (no Bungie art is copied). */
export function demoEmblem(seed: number): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 36 36"><rect width="36" height="36" fill="${GREYS[seed % GREYS.length]}"/>${MARKS[seed % MARKS.length]}</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

const isoAgo = (now: number, ms: number) => new Date(now - ms).toISOString();

interface Seed {
  id: string;
  name: string;
  status: (now: number) => Status;
}

// Made-up players; the landing copy labels the board as a sample.
const SEEDS: Seed[] = [
  { id: 'demo-rook', name: 'Rook#2093', status: (n) => ({ state: 'IN_MATCH', playlist: 'Trials of Osiris', activity: 'Burnout', startedAt: isoAgo(n, 252_000) }) },
  { id: 'demo-mire', name: 'Mire#3307', status: (n) => ({ state: 'LIKELY_QUEUING', place: 'ORBIT', lastPvpPlaylist: 'Competitive', lastPvpEndedAt: isoAgo(n, 70_000) }) },
  { id: 'demo-kestrel', name: 'Kestrel#0412', status: (n) => ({ state: 'NOT_IN_PVP', place: 'ORBIT', activity: 'Orbit', lastPvpEndedAt: isoAgo(n, 3_600_000) }) },
  { id: 'demo-halfmoon', name: 'Halfmoon#7781', status: () => ({ state: 'NOT_IN_PVP', place: 'PVE', activity: 'Vanguard Ops' }) },
  { id: 'demo-tallow', name: 'Tallow#5510', status: (n) => ({ state: 'OFFLINE', lastPvpEndedAt: isoAgo(n, 2 * 86_400_000) }) },
  { id: 'demo-vantage', name: 'Vantage#1188', status: () => ({ state: 'PRIVATE' }) },
];

export function demoPlayers(now: number): Player[] {
  return SEEDS.map((s, i) => ({
    membershipType: 3,
    membershipId: s.id,
    bungieName: s.name,
    emblemPath: demoEmblem(i),
    emblemBackgroundPath: '',
    status: s.status(now),
    lastPolledAt: isoAgo(now, 5_000),
    lastSuccessAt: isoAgo(now, 5_000),
    source: 'match',
    addedAt: isoAgo(now, 86_400_000),
  }));
}

type Step = (now: number) => PlayerUpdate[];
const update = (membershipId: string, status: Status, now: number): PlayerUpdate => ({
  membershipId,
  status,
  polledAt: new Date(now).toISOString(),
});

/** One step every DEMO_STEP_MS: the loop a watcher would see over a few minutes. */
export const DEMO_STEPS: readonly Step[] = [
  (n) => [update('demo-kestrel', { state: 'IN_MATCH', playlist: 'Competitive', activity: 'Javelin-4', startedAt: new Date(n).toISOString() }, n)],
  (n) => [update('demo-rook', { state: 'LIKELY_QUEUING', place: 'ORBIT', lastPvpPlaylist: 'Trials of Osiris', lastPvpEndedAt: new Date(n).toISOString() }, n)],
  (n) => [update('demo-mire', { state: 'IN_MATCH', playlist: 'Competitive', activity: 'Burnout', startedAt: new Date(n).toISOString() }, n)],
  (n) => [update('demo-tallow', { state: 'NOT_IN_PVP', place: 'ORBIT', activity: 'Orbit' }, n)],
  (n) => [update('demo-rook', { state: 'IN_MATCH', playlist: 'Trials of Osiris', activity: 'Endless Vale', startedAt: new Date(n).toISOString() }, n)],
];

/** A scripted stream that can be paused and resumed in place; `play` is idempotent. */
export function createDemo(dispatch: (a: BoardAction) => void, clock: () => number = Date.now) {
  const reset = () => dispatch({ type: 'snapshot', snapshot: { players: demoPlayers(clock()), apiStatus: 'ok' } });
  let step = 0;
  let id: ReturnType<typeof setInterval> | undefined;
  const tick = () => {
    if (step >= DEMO_STEPS.length) {
      step = 0;
      reset();
      return;
    }
    for (const u of DEMO_STEPS[step](clock())) dispatch({ type: 'player_update', update: u });
    step += 1;
  };
  return {
    play() {
      if (id === undefined) id = setInterval(tick, DEMO_STEP_MS);
    },
    pause() {
      clearInterval(id);
      id = undefined;
    },
  };
}

/** Feeds the board a scripted stream; returns a stop function. Loops back to the opening board. */
export function startDemo(dispatch: (a: BoardAction) => void, clock: () => number = Date.now): () => void {
  dispatch({ type: 'snapshot', snapshot: { players: demoPlayers(clock()), apiStatus: 'ok' } });
  const demo = createDemo(dispatch, clock);
  demo.play();
  return demo.pause;
}
