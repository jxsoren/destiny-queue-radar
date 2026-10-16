import { describe, expect, test } from 'vitest';
import { iso, makePlayer } from '../test/fixtures';
import { boardReducer, initialBoard, type BoardState } from './reducer';

const withPlayers = (...ps: ReturnType<typeof makePlayer>[]): BoardState =>
  boardReducer(initialBoard, { type: 'snapshot', snapshot: { players: ps, apiStatus: 'ok' } });

describe('boardReducer', () => {
  test('starts empty, not loaded, not connected', () => {
    expect(initialBoard).toMatchObject({ apiStatus: 'ok', connected: false, loaded: false });
    expect(initialBoard.players.size).toBe(0);
  });

  test('snapshot replaces everything and marks connected', () => {
    const before = withPlayers(makePlayer({ membershipId: 'old' }));
    const after = boardReducer(before, { type: 'snapshot', snapshot: { players: [makePlayer({ membershipId: 'new' })], apiStatus: 'degraded' } });
    expect([...after.players.keys()]).toEqual(['new']);
    expect(after).toMatchObject({ apiStatus: 'degraded', connected: true, loaded: true });
  });

  test('player_update merges status and poll times', () => {
    const s = withPlayers(makePlayer({ membershipId: 'a', bungieName: 'A#0001' }));
    const polledAt = iso(1_000);
    const next = boardReducer(s, { type: 'player_update', update: { membershipId: 'a', status: { state: 'IN_MATCH', activity: 'Burnout' }, polledAt } });
    expect(next.players.get('a')).toMatchObject({
      bungieName: 'A#0001',
      status: { state: 'IN_MATCH', activity: 'Burnout' },
      lastPolledAt: polledAt,
      lastSuccessAt: polledAt,
    });
    expect(s.players.get('a')?.status.state).toBe('NOT_IN_PVP'); // previous state untouched
  });

  test('player_update for an unknown player is ignored', () => {
    const s = withPlayers();
    expect(boardReducer(s, { type: 'player_update', update: { membershipId: 'x', status: { state: 'OFFLINE' }, polledAt: iso(0) } })).toBe(s);
  });

  test('api_status sets the Bungie status', () => {
    expect(boardReducer(initialBoard, { type: 'api_status', status: 'down' }).apiStatus).toBe('down');
  });

  test('watchlist adds new players and keeps a newer streamed status', () => {
    const streamed = makePlayer({ membershipId: 'a', status: { state: 'IN_MATCH' }, lastPolledAt: iso(1_000), lastSuccessAt: iso(1_000) });
    const s = withPlayers(streamed);
    const fetched = [
      makePlayer({ membershipId: 'a', bungieName: 'A#0001', status: { state: 'NOT_IN_PVP' }, lastPolledAt: iso(60_000) }),
      makePlayer({ membershipId: 'b', bungieName: 'B#0002', status: { state: '' }, lastPolledAt: null, lastSuccessAt: null }),
    ];
    const next = boardReducer(s, { type: 'watchlist', players: fetched });
    expect([...next.players.keys()].sort()).toEqual(['a', 'b']);
    expect(next.players.get('a')).toMatchObject({ bungieName: 'A#0001', status: { state: 'IN_MATCH' } });
    expect(next.loaded).toBe(true);
  });

  test('watchlist drops players no longer listed', () => {
    const s = withPlayers(makePlayer({ membershipId: 'gone' }));
    expect(boardReducer(s, { type: 'watchlist', players: [] }).players.size).toBe(0);
  });

  test('remove and disconnected', () => {
    const s = withPlayers(makePlayer({ membershipId: 'a' }));
    expect(boardReducer(s, { type: 'remove', membershipId: 'a' }).players.has('a')).toBe(false);
    expect(boardReducer(s, { type: 'disconnected' }).connected).toBe(false);
  });
});
