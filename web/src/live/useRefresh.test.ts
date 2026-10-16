import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { ApiError } from '../api/client';
import { REFRESH_COOLDOWN_MS, useRefresh } from './useRefresh';

const T0 = Date.parse('2026-10-15T18:00:00Z');

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(T0);
});
afterEach(() => vi.useRealTimers());

function setup(refresh: () => Promise<void>) {
  const onBungieDown = vi.fn();
  const hook = renderHook(() => useRefresh({ client: { refresh }, onBungieDown }));
  return { hook, onBungieDown };
}

test('202 starts a 30s cooldown', async () => {
  const { hook } = setup(vi.fn().mockResolvedValue(undefined));
  await act(() => hook.result.current.refresh());
  expect(hook.result.current.cooldownUntil).toBe(T0 + REFRESH_COOLDOWN_MS);
  expect(hook.result.current.busy).toBe(false);
});

test('429 uses Retry-After', async () => {
  const { hook } = setup(vi.fn().mockRejectedValue(new ApiError(429, 'refresh is on cooldown', 12, false)));
  await act(() => hook.result.current.refresh());
  expect(hook.result.current.cooldownUntil).toBe(T0 + 12_000);
});

test('503 reports Bungie down without a cooldown', async () => {
  const { hook, onBungieDown } = setup(vi.fn().mockRejectedValue(new ApiError(503, 'down', null, false)));
  await act(() => hook.result.current.refresh());
  expect(onBungieDown).toHaveBeenCalledOnce();
  expect(hook.result.current.cooldownUntil).toBe(0);
});

test('other failures change nothing', async () => {
  const { hook, onBungieDown } = setup(vi.fn().mockRejectedValue(new ApiError(500, 'internal error', null, false)));
  await act(() => hook.result.current.refresh());
  expect(onBungieDown).not.toHaveBeenCalled();
  expect(hook.result.current.cooldownUntil).toBe(0);
});

test('a second call while one is in flight is a no-op', async () => {
  let resolve!: () => void;
  const refresh = vi.fn().mockImplementationOnce(() => new Promise<void>((r) => (resolve = r))).mockResolvedValue(undefined);
  const { hook } = setup(refresh);
  let first!: Promise<void>;
  act(() => {
    first = hook.result.current.refresh();
    void hook.result.current.refresh();
  });
  expect(refresh).toHaveBeenCalledOnce();
  await act(async () => {
    resolve();
    await first;
  });
  await act(() => hook.result.current.refresh());
  expect(refresh).toHaveBeenCalledTimes(2);
});
