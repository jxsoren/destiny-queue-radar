import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { columnOf } from '../../board/columns';
import { boardReducer, initialBoard, type BoardAction, type BoardState } from '../../board/reducer';
import { DEMO_STEPS, DEMO_STEP_MS, demoEmblem, demoPlayers, startDemo } from './script';

const T0 = Date.parse('2026-10-15T18:00:00Z');

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(T0);
});
afterEach(() => vi.useRealTimers());

test('six made-up players fill all four columns', () => {
  const players = demoPlayers(T0);
  expect(players).toHaveLength(6);
  expect(new Set(players.map((p) => columnOf(p.status.state)))).toEqual(new Set(['match', 'queue', 'idle', 'offline']));
  for (const p of players) expect(p.emblemPath.startsWith('data:image/svg+xml,')).toBe(true);
});

test('emblems are drawn locally, never from bungie.net', () => {
  expect(demoEmblem(0)).not.toContain('bungie.net');
  expect(demoEmblem(1)).not.toBe(demoEmblem(2));
});

test('the script moves players through the columns and loops', () => {
  let state: BoardState = initialBoard;
  const dispatch = (a: BoardAction) => {
    state = boardReducer(state, a);
  };
  const stop = startDemo(dispatch);
  const column = (id: string) => columnOf(state.players.get(id)!.status.state);
  expect(column('demo-kestrel')).toBe('idle');

  vi.advanceTimersByTime(DEMO_STEP_MS);
  expect(column('demo-kestrel')).toBe('match');

  vi.advanceTimersByTime(DEMO_STEP_MS * (DEMO_STEPS.length - 1));
  vi.advanceTimersByTime(DEMO_STEP_MS); // loop: back to the opening board
  expect(column('demo-kestrel')).toBe('idle');

  stop();
  const before = state;
  vi.advanceTimersByTime(DEMO_STEP_MS * 3);
  expect(state).toBe(before);
});

test('banners are drawn locally too, at the emblem banner shape', () => {
  for (const p of demoPlayers(Date.now())) {
    expect(p.emblemBackgroundPath.startsWith('data:image/svg+xml,')).toBe(true);
    expect(decodeURIComponent(p.emblemBackgroundPath)).toContain('viewBox="0 0 474 96"');
  }
});
