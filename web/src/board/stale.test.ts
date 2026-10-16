import { expect, test } from 'vitest';
import { NOW, iso, makePlayer } from '../test/fixtures';
import { latestSuccess, staleFor } from './stale';

test('fresh within 3 minutes', () => {
  expect(staleFor(makePlayer({ lastSuccessAt: iso(179_000) }), NOW)).toBeNull();
});

test('stale after 3 minutes, aged from the last success', () => {
  expect(staleFor(makePlayer({ lastSuccessAt: iso(240_000), lastPolledAt: iso(5_000) }), NOW)).toBe(240_000);
});

test('never polled is checking, not stale', () => {
  expect(staleFor(makePlayer({ lastSuccessAt: null, lastPolledAt: null }), NOW)).toBeNull();
});

test('polled but never succeeded is stale since it was added', () => {
  expect(staleFor(makePlayer({ lastSuccessAt: null, lastPolledAt: iso(5_000), addedAt: iso(600_000) }), NOW)).toBe(600_000);
});

test('latestSuccess is the newest success, or null', () => {
  expect(latestSuccess([])).toBeNull();
  expect(
    latestSuccess([makePlayer({ lastSuccessAt: iso(60_000) }), makePlayer({ lastSuccessAt: iso(5_000) }), makePlayer({ lastSuccessAt: null })]),
  ).toBe(NOW - 5_000);
});
