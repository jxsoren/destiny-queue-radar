import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { ApiError } from '../api/client';
import { FakeStream, fakeStreamFactory } from '../test/fakeStream';
import { makePlayer } from '../test/fixtures';
import { useBoard } from './useBoard';

function setup() {
  const client = {
    me: vi.fn().mockResolvedValue({ id: 1, bungieName: 'Me#0001', membershipType: 3, membershipId: '1' }),
    watchlist: vi.fn().mockResolvedValue([makePlayer({ membershipId: 'w' })]),
  };
  const onSignedOut = vi.fn();
  const hook = renderHook(() => useBoard({ onSignedOut, stream: fakeStreamFactory, client }));
  return { client, onSignedOut, hook };
}

beforeEach(() => {
  FakeStream.reset();
  vi.useFakeTimers();
});
afterEach(() => vi.useRealTimers());

describe('useBoard', () => {
  test('opens /api/stream and applies snapshot, updates and api_status', () => {
    const { hook } = setup();
    const s = FakeStream.latest();
    expect(s.url).toBe('/api/stream');
    act(() => s.emit('snapshot', { players: [makePlayer({ membershipId: 'a' })], apiStatus: 'ok' }));
    expect(hook.result.current.state.connected).toBe(true);
    expect(hook.result.current.state.players.has('a')).toBe(true);

    act(() => s.emit('player_update', { membershipId: 'a', status: { state: 'IN_MATCH' }, polledAt: '2026-10-15T18:00:00Z' }));
    expect(hook.result.current.state.players.get('a')?.status.state).toBe('IN_MATCH');

    act(() => s.emit('api_status', { bungie: 'down' }));
    expect(hook.result.current.state.apiStatus).toBe('down');
  });

  test('malformed frames are ignored without throwing', () => {
    const { hook } = setup();
    const s = FakeStream.latest();
    act(() => s.emit('snapshot', { players: [makePlayer({ membershipId: 'a' })], apiStatus: 'ok' }));
    const before = hook.result.current.state;
    // JSON.stringify(undefined) is undefined, so the frame's data is the unparseable text "undefined".
    for (const type of ['snapshot', 'player_update', 'api_status']) {
      expect(() => act(() => s.emit(type, undefined))).not.toThrow();
    }
    expect(hook.result.current.state).toBe(before);
  });

  test('logout closes the stream and signs out', () => {
    const { onSignedOut } = setup();
    const s = FakeStream.latest();
    act(() => s.emit('logout', {}));
    expect(s.closed).toBe(true);
    expect(onSignedOut).toHaveBeenCalledOnce();
  });

  test('an error after a snapshot leaves reconnecting to the browser', async () => {
    const { client, hook } = setup();
    const s = FakeStream.latest();
    act(() => s.emit('snapshot', { players: [], apiStatus: 'ok' }));
    act(() => s.fail(0));
    expect(s.closed).toBe(false);
    expect(hook.result.current.state.connected).toBe(false);
    await act(() => vi.advanceTimersByTimeAsync(60_000));
    expect(client.me).not.toHaveBeenCalled();
    expect(FakeStream.all).toHaveLength(1);
  });

  test('a failure before any snapshot with an ended session signs out', async () => {
    const { client, onSignedOut } = setup();
    client.me.mockRejectedValue(new ApiError(401, 'sign in required', null, false));
    act(() => FakeStream.latest().fail());
    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(onSignedOut).toHaveBeenCalledOnce();
    await act(() => vi.advanceTimersByTimeAsync(60_000));
    expect(FakeStream.all).toHaveLength(1);
  });

  test('a failure while signed in shows too many tabs, loads the watchlist and backs off', async () => {
    const { client, hook } = setup();
    act(() => FakeStream.latest().fail());
    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(hook.result.current.tooManyTabs).toBe(true);
    expect(client.watchlist).toHaveBeenCalledOnce();
    expect(hook.result.current.state.players.has('w')).toBe(true);

    await act(() => vi.advanceTimersByTimeAsync(9_999));
    expect(FakeStream.all).toHaveLength(1);
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(FakeStream.all).toHaveLength(2);

    act(() => FakeStream.latest().fail());
    await act(() => vi.advanceTimersByTimeAsync(19_999));
    expect(FakeStream.all).toHaveLength(2);
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(FakeStream.all).toHaveLength(3);

    act(() => FakeStream.latest().emit('snapshot', { players: [], apiStatus: 'ok' }));
    expect(hook.result.current.tooManyTabs).toBe(false);
  });

  test('a network failure on /api/me retries without the tabs notice', async () => {
    const { client, hook } = setup();
    client.me.mockRejectedValue(new TypeError('Failed to fetch'));
    act(() => FakeStream.latest().fail());
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(hook.result.current.tooManyTabs).toBe(false);
    expect(FakeStream.all).toHaveLength(2);
  });

  test('unmount closes the stream and cancels retries', async () => {
    const { hook } = setup();
    const s = FakeStream.latest();
    hook.unmount();
    expect(s.closed).toBe(true);
    await act(() => vi.advanceTimersByTimeAsync(60_000));
    expect(FakeStream.all).toHaveLength(1);
  });

  test('refetch loads the watchlist', async () => {
    const { client, hook } = setup();
    await act(() => hook.result.current.refetch());
    expect(client.watchlist).toHaveBeenCalledOnce();
    expect(hook.result.current.state.players.has('w')).toBe(true);
  });
});
