import { describe, expect, test } from 'vitest';
import { ApiError } from '../api/client';
import { needsReauth, paletteError } from './errors';

const err = (status: number, retryAfter: number | null = null, reauth = false) => new ApiError(status, 'x', retryAfter, reauth);

describe('paletteError (spec §4 table)', () => {
  test.each([
    [err(404), 'add', 'PLAYER NOT FOUND'],
    [err(404), 'search', 'PLAYER NOT FOUND'],
    [err(409), 'add', 'WATCHLIST FULL (50)'],
    [err(429, 12), 'add', 'SLOW DOWN — RETRY IN 12S'],
    [err(429), 'search', 'SLOW DOWN — RETRY IN 60S'],
    [err(503), 'add', 'BUNGIE UNAVAILABLE'],
    [err(502), 'search', 'COULD NOT REACH BUNGIE'],
    [err(401, null, true), 'matches', 'SIGN IN AGAIN TO SEE YOUR MATCHES'],
    [err(401, null, true), 'add', 'SIGN IN AGAIN'],
    [err(409), 'matches', 'NO DESTINY 2 ACCOUNT LINKED · SIGN IN AGAIN'],
    [err(400), 'search', 'ENTER A BUNGIE NAME LIKE NAME#1234'],
    [err(500), 'add', 'SOMETHING WENT WRONG'],
    [new TypeError('Failed to fetch'), 'add', 'COULD NOT REACH QUEUE RADAR'],
  ] as const)('%o in %s → %s', (e, ctx, want) => expect(paletteError(e, ctx)).toBe(want));
});

test('needsReauth', () => {
  expect(needsReauth(err(401, null, true))).toBe(true);
  expect(needsReauth(err(401))).toBe(false);
  expect(needsReauth(new Error('x'))).toBe(false);
});
