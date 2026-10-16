import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { ApiError, api, setUnauthorizedHandler } from './client';

const fetchMock = vi.fn<typeof fetch>();

function respond(status: number, body?: unknown, headers: Record<string, string> = {}): Response {
  const text = body === undefined ? null : JSON.stringify(body);
  return new Response(text, { status, headers: { 'Content-Type': 'application/json', ...headers } });
}

beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock);
  fetchMock.mockReset();
});
afterEach(() => {
  vi.unstubAllGlobals();
  setUnauthorizedHandler(null);
});

describe('requests', () => {
  test('GET sends credentials and no CSRF header, and parses JSON', async () => {
    fetchMock.mockResolvedValue(respond(200, [{ membershipId: '1' }]));
    const list = await api.watchlist();
    expect(list).toEqual([{ membershipId: '1' }]);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/watchlist');
    expect(init?.method).toBe('GET');
    expect(init?.credentials).toBe('same-origin');
    expect((init?.headers as Record<string, string>)['X-Requested-With']).toBeUndefined();
  });

  test('mutations send X-Requested-With: dqr and a JSON body; 201 resolves to undefined', async () => {
    fetchMock.mockResolvedValue(respond(201));
    await expect(api.addWatch({ membershipType: 3, membershipId: '4611', source: 'match' })).resolves.toBeUndefined();
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/watchlist');
    expect(init?.method).toBe('POST');
    const headers = init?.headers as Record<string, string>;
    expect(headers['X-Requested-With']).toBe('dqr');
    expect(headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(init?.body as string)).toEqual({ membershipType: 3, membershipId: '4611', source: 'match' });
  });

  test('DELETE encodes the id and sends the CSRF header', async () => {
    fetchMock.mockResolvedValue(respond(204));
    await api.removeWatch('4611');
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/watchlist/4611');
    expect(init?.method).toBe('DELETE');
    expect((init?.headers as Record<string, string>)['X-Requested-With']).toBe('dqr');
  });

  test('search encodes the Bungie Name', async () => {
    fetchMock.mockResolvedValue(respond(200, { membershipId: '7' }));
    await api.search('Ghost™#0880');
    expect(fetchMock.mock.calls[0][0]).toBe('/api/search?q=Ghost%E2%84%A2%230880');
  });
});

describe('errors', () => {
  test('a rejected fetch becomes an ApiError with status 0', async () => {
    fetchMock.mockRejectedValue(new TypeError('Failed to fetch'));
    const err = await api.watchlist().catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 0, message: 'network error', retryAfter: null, reauth: false });
  });

  test('a 200 with a non-JSON body becomes an ApiError', async () => {
    fetchMock.mockResolvedValue(new Response('<html>oops</html>', { status: 200 }));
    const err = await api.watchlist().catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 200, message: 'invalid response' });
  });

  test('carry status, message and Retry-After', async () => {
    fetchMock.mockResolvedValue(respond(429, { error: 'too many lookups; slow down' }, { 'Retry-After': '12' }));
    const err = await api.search('A#0001').catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 429, message: 'too many lookups; slow down', retryAfter: 12, reauth: false });
  });

  test('a non-JSON error body still gives an ApiError', async () => {
    fetchMock.mockResolvedValue(new Response('bad gateway', { status: 502 }));
    await expect(api.watchlist()).rejects.toMatchObject({ status: 502, retryAfter: null });
  });

  test('401 calls the unauthorized handler', async () => {
    const onUnauthorized = vi.fn();
    setUnauthorizedHandler(onUnauthorized);
    fetchMock.mockResolvedValue(respond(401, { error: 'sign in required' }));
    await expect(api.watchlist()).rejects.toMatchObject({ status: 401 });
    expect(onUnauthorized).toHaveBeenCalledOnce();
  });

  test('401 from /api/me and reauth 401s do not call the handler', async () => {
    const onUnauthorized = vi.fn();
    setUnauthorizedHandler(onUnauthorized);
    fetchMock.mockResolvedValueOnce(respond(401, { error: 'sign in required' }));
    await expect(api.me()).rejects.toMatchObject({ status: 401 });
    fetchMock.mockResolvedValueOnce(respond(401, { error: 'sign in again to see your matches', reauth: true }));
    await expect(api.recentMatches()).rejects.toMatchObject({ status: 401, reauth: true });
    expect(onUnauthorized).not.toHaveBeenCalled();
  });
});
