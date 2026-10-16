import type { AddWatchRequest, Match, Me, Player, SearchResult } from './types';

/** A non-2xx answer from the server ({"error": "..."} bodies). */
export class ApiError extends Error {
  readonly status: number;
  /** Seconds from the Retry-After header, when present. */
  readonly retryAfter: number | null;
  /** True on a 401 from recent matches that only needs a new Bungie sign-in. */
  readonly reauth: boolean;

  constructor(status: number, message: string, retryAfter: number | null, reauth: boolean) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.retryAfter = retryAfter;
    this.reauth = reauth;
  }
}

let onUnauthorized: (() => void) | null = null;

/** Registers what to do when a request finds the session gone (the App routes to Landing). */
export function setUnauthorizedHandler(fn: (() => void) | null): void {
  onUnauthorized = fn;
}

interface RequestOptions {
  body?: unknown;
  /** Don't call the unauthorized handler on 401 (the caller decides). */
  quiet401?: boolean;
}

/** Every failure is an ApiError; status 0 means the server was unreachable. */
async function request<T>(method: string, path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (method !== 'GET') headers['X-Requested-With'] = 'dqr'; // CSRF header (main spec §7.3)
  let body: string | undefined;
  if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json';
    body = JSON.stringify(opts.body);
  }
  let res: Response;
  try {
    res = await fetch(path, { method, headers, body, credentials: 'same-origin' });
  } catch {
    throw new ApiError(0, 'network error', null, false);
  }
  if (!res.ok) {
    const data: { error?: unknown; reauth?: unknown } = await res.json().catch(() => ({}));
    const ra = res.headers.get('Retry-After');
    const retryAfter = ra !== null && /^\d+$/.test(ra) ? Number(ra) : null;
    const message = typeof data.error === 'string' ? data.error : res.statusText;
    const err = new ApiError(res.status, message, retryAfter, data.reauth === true);
    if (res.status === 401 && !err.reauth && !opts.quiet401) onUnauthorized?.();
    throw err;
  }
  if (res.status === 201 || res.status === 202 || res.status === 204) return undefined as T;
  try {
    return (await res.json()) as T;
  } catch {
    throw new ApiError(res.status, 'invalid response', null, false);
  }
}

export const api = {
  me: () => request<Me>('GET', '/api/me', { quiet401: true }),
  watchlist: () => request<Player[]>('GET', '/api/watchlist'),
  addWatch: (req: AddWatchRequest) => request<void>('POST', '/api/watchlist', { body: req }),
  removeWatch: (membershipId: string) =>
    request<void>('DELETE', `/api/watchlist/${encodeURIComponent(membershipId)}`),
  search: (bungieName: string) => request<SearchResult>('GET', `/api/search?q=${encodeURIComponent(bungieName)}`),
  recentMatches: () => request<Match[]>('GET', '/api/matches/recent'),
  refresh: () => request<void>('POST', '/api/refresh'),
  logout: () => request<void>('POST', '/auth/logout', { quiet401: true }),
};

export type Api = typeof api;
