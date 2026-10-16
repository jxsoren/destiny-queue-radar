import { describe, expect, test } from 'vitest';
import type { State } from '../api/types';
import { iso, makePlayer } from '../test/fixtures';
import { columnOf, groupPlayers } from './columns';

describe('columnOf', () => {
  test.each<[State, string]>([
    ['IN_MATCH', 'match'],
    ['LIKELY_QUEUING', 'queue'],
    ['NOT_IN_PVP', 'idle'],
    ['', 'idle'],
    ['OFFLINE', 'offline'],
    ['PRIVATE', 'offline'],
    ['NOT_FOUND', 'offline'],
  ])('%s → %s', (state, col) => expect(columnOf(state)).toBe(col));
});

describe('groupPlayers', () => {
  test('in match: longest in match first; missing start last', () => {
    const g = groupPlayers([
      makePlayer({ membershipId: 'short', status: { state: 'IN_MATCH', startedAt: iso(60_000) } }),
      makePlayer({ membershipId: 'nostart', status: { state: 'IN_MATCH' } }),
      makePlayer({ membershipId: 'long', status: { state: 'IN_MATCH', startedAt: iso(600_000) } }),
    ]);
    expect(g.match.map((p) => p.membershipId)).toEqual(['long', 'short', 'nostart']);
  });

  test('queuing: most recently ended first', () => {
    const g = groupPlayers([
      makePlayer({ membershipId: 'old', status: { state: 'LIKELY_QUEUING', lastPvpEndedAt: iso(240_000) } }),
      makePlayer({ membershipId: 'new', status: { state: 'LIKELY_QUEUING', lastPvpEndedAt: iso(30_000) } }),
    ]);
    expect(g.queue.map((p) => p.membershipId)).toEqual(['new', 'old']);
  });

  test('others: by name, case-insensitive, unnamed last', () => {
    const g = groupPlayers([
      makePlayer({ membershipId: '1', bungieName: 'zed#0001', status: { state: 'OFFLINE' } }),
      makePlayer({ membershipId: '2', bungieName: '', status: { state: 'PRIVATE' } }),
      makePlayer({ membershipId: '3', bungieName: 'Alpha#0002', status: { state: 'NOT_FOUND' } }),
      makePlayer({ membershipId: '4', bungieName: 'beta#0003', status: { state: '' } }),
    ]);
    expect(g.offline.map((p) => p.membershipId)).toEqual(['3', '1', '2']);
    expect(g.idle.map((p) => p.membershipId)).toEqual(['4']);
    expect(g.match).toEqual([]);
  });
});
