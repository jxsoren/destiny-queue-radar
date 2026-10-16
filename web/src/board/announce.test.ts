import { expect, test } from 'vitest';
import { makePlayer } from '../test/fixtures';
import { columnChanges, columnMap } from './announce';

test('reports players whose column changed, not new players', () => {
  const before = columnMap([makePlayer({ membershipId: 'a', bungieName: 'Vex#1111', status: { state: 'NOT_IN_PVP' } })]);
  const after = [
    makePlayer({ membershipId: 'a', bungieName: 'Vex#1111', status: { state: 'IN_MATCH' } }),
    makePlayer({ membershipId: 'b', bungieName: 'New#0002', status: { state: 'IN_MATCH' } }),
  ];
  expect(columnChanges(before, after)).toEqual(['VEX#1111 · IN MATCH']);
});

test('no message when the column is the same', () => {
  const before = columnMap([makePlayer({ membershipId: 'a', status: { state: 'OFFLINE' } })]);
  expect(columnChanges(before, [makePlayer({ membershipId: 'a', status: { state: 'PRIVATE' } })])).toEqual([]);
});
