import { describe, expect, test } from 'vitest';
import { formatAgo, formatTimer, pad2, since } from './format';

describe('formatTimer', () => {
  test.each([
    [0, '00:00'],
    [999, '00:00'],
    [252_000, '04:12'],
    [3_599_000, '59:59'],
    [3_600_000, '1:00:00'],
    [3_723_000, '1:02:03'],
    [-5_000, '00:00'],
  ])('%i ms → %s', (ms, want) => expect(formatTimer(ms)).toBe(want));
});

describe('formatAgo', () => {
  test.each([
    [0, '0S'],
    [23_000, '23S'],
    [59_999, '59S'],
    [60_000, '1M'],
    [120_000, '2M'],
    [3_599_000, '59M'],
    [3_600_000, '1H'],
    [86_399_000, '23H'],
    [86_400_000, '1D'],
    [2 * 86_400_000, '2D'],
    [-1_000, '0S'],
  ])('%i ms → %s', (ms, want) => expect(formatAgo(ms)).toBe(want));
});

test('pad2', () => {
  expect(pad2(7)).toBe('07');
  expect(pad2(50)).toBe('50');
  expect(pad2(123)).toBe('123');
});

test('since', () => {
  const now = Date.parse('2026-10-15T18:00:00Z');
  expect(since('2026-10-15T17:58:00Z', now)).toBe(120_000);
  expect(since(null, now)).toBeNull();
  expect(since(undefined, now)).toBeNull();
  expect(since('not a date', now)).toBeNull();
});
