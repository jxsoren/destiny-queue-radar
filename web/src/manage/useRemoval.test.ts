import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makePlayer } from '../test/fixtures';
import { UNDO_MS, useRemoval } from './useRemoval';

const a = makePlayer({ membershipId: 'a', bungieName: 'A#0001' });
const b = makePlayer({ membershipId: 'b', bungieName: 'B#0002' });

function setup(remove = vi.fn().mockResolvedValue(undefined)) {
  const onRemoved = vi.fn();
  const onFailed = vi.fn();
  const hook = renderHook(() => useRemoval({ remove, onRemoved, onFailed }));
  return { hook, remove, onRemoved, onFailed };
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

test('hides at once and deletes after 5 seconds', async () => {
  const { hook, remove, onRemoved } = setup();
  act(() => hook.result.current.start(a));
  expect(hook.result.current.hidden.has('a')).toBe(true);
  expect(hook.result.current.pending).toBe(a);
  await act(() => vi.advanceTimersByTimeAsync(UNDO_MS - 1));
  expect(remove).not.toHaveBeenCalled();
  await act(() => vi.advanceTimersByTimeAsync(1));
  expect(remove).toHaveBeenCalledWith('a');
  expect(onRemoved).toHaveBeenCalledWith('a');
  expect(hook.result.current.pending).toBeNull();
  expect(hook.result.current.hidden.has('a')).toBe(false);
});

test('undo cancels the delete and shows the card again', async () => {
  const { hook, remove } = setup();
  act(() => hook.result.current.start(a));
  act(() => hook.result.current.undo());
  expect(hook.result.current.hidden.has('a')).toBe(false);
  expect(hook.result.current.pending).toBeNull();
  await act(() => vi.advanceTimersByTimeAsync(UNDO_MS * 2));
  expect(remove).not.toHaveBeenCalled();
});

test('a second removal sends the first delete immediately', async () => {
  const { hook, remove } = setup();
  act(() => hook.result.current.start(a));
  act(() => hook.result.current.start(b));
  expect(remove).toHaveBeenCalledTimes(1);
  expect(remove).toHaveBeenCalledWith('a');
  expect(hook.result.current.pending).toBe(b);
  await act(() => vi.advanceTimersByTimeAsync(UNDO_MS));
  expect(remove).toHaveBeenLastCalledWith('b');
});

test('a failed delete restores the card and reports', async () => {
  const failure = new Error('500');
  const { hook, onFailed, onRemoved } = setup(vi.fn().mockRejectedValue(failure));
  act(() => hook.result.current.start(a));
  await act(() => vi.advanceTimersByTimeAsync(UNDO_MS));
  expect(hook.result.current.hidden.has('a')).toBe(false);
  expect(onFailed).toHaveBeenCalledWith(a, failure);
  expect(onRemoved).not.toHaveBeenCalled();
});

test('unmounting sends a pending delete', () => {
  const { hook, remove } = setup();
  act(() => hook.result.current.start(a));
  hook.unmount();
  expect(remove).toHaveBeenCalledWith('a');
});
