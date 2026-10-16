import { ApiError } from '../api/client';

export type ErrorContext = 'add' | 'search' | 'matches';

/** The inline palette messages from spec §4. */
export function paletteError(err: unknown, ctx: ErrorContext): string {
  if (!(err instanceof ApiError)) return 'COULD NOT REACH QUEUE RADAR';
  switch (err.status) {
    case 401:
      return err.reauth ? 'SIGN IN AGAIN TO SEE YOUR MATCHES' : 'SIGNED OUT';
    case 404:
      return 'PLAYER NOT FOUND';
    case 409:
      return ctx === 'matches' ? 'NO DESTINY 2 ACCOUNT LINKED · SIGN IN AGAIN' : 'WATCHLIST FULL (50)';
    case 429:
      return `SLOW DOWN — RETRY IN ${err.retryAfter ?? 60}S`;
    case 503:
      return 'BUNGIE UNAVAILABLE';
    case 502:
      return 'COULD NOT REACH BUNGIE';
    case 400:
      return ctx === 'search' ? 'ENTER A BUNGIE NAME LIKE NAME#1234' : 'INVALID REQUEST';
    default:
      return 'SOMETHING WENT WRONG';
  }
}

/** A recent-matches 401 that only needs a fresh Bungie sign-in. */
export function needsReauth(err: unknown): boolean {
  return err instanceof ApiError && err.status === 401 && err.reauth;
}
