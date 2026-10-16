import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, test, vi } from 'vitest';
import { ApiError, type Api } from './api/client';
import { App } from './App';
import { FakeStream, fakeStreamFactory } from './test/fakeStream';

const me = { id: 1, bungieName: 'Tester#0001', membershipType: 3, membershipId: 'me' };

function client(meResult: () => Promise<unknown>) {
  return {
    me: vi.fn(meResult),
    watchlist: vi.fn().mockResolvedValue([]),
    addWatch: vi.fn(),
    removeWatch: vi.fn(),
    search: vi.fn(),
    recentMatches: vi.fn().mockResolvedValue([]),
    refresh: vi.fn(),
    logout: vi.fn().mockResolvedValue(undefined),
  } as unknown as Api;
}

beforeEach(() => FakeStream.reset());

test('signed in: the board', async () => {
  render(<App client={client(() => Promise.resolve(me))} stream={fakeStreamFactory} />);
  expect(await screen.findByRole('button', { name: 'SIGN OUT' })).toBeInTheDocument();
});

test('signed out: the landing page with Bungie sign-in', async () => {
  render(<App client={client(() => Promise.reject(new ApiError(401, 'sign in required', null, false)))} stream={fakeStreamFactory} />);
  const signIn = await screen.findAllByRole('link', { name: /sign in with bungie/i });
  for (const link of signIn) expect(link).toHaveAttribute('href', '/auth/login');
  expect(FakeStream.all).toHaveLength(0);
});

test('a server error offers a retry instead of the landing page', async () => {
  const user = userEvent.setup();
  const meFn = vi.fn().mockRejectedValueOnce(new ApiError(500, 'internal error', null, false)).mockResolvedValueOnce(me);
  render(<App client={client(meFn)} stream={fakeStreamFactory} />);
  await user.click(await screen.findByRole('button', { name: 'RETRY' }));
  expect(await screen.findByRole('button', { name: 'SIGN OUT' })).toBeInTheDocument();
});

test('a logout event on the stream routes to the landing page', async () => {
  render(<App client={client(() => Promise.resolve(me))} stream={fakeStreamFactory} />);
  await screen.findByRole('button', { name: 'SIGN OUT' });
  act(() => FakeStream.latest().emit('logout', {}));
  expect(await screen.findByRole('heading', { level: 1, name: /queue while the cheater plays/i })).toBeInTheDocument();
});

test('sign out routes to the landing page', async () => {
  const user = userEvent.setup();
  const c = client(() => Promise.resolve(me));
  render(<App client={c} stream={fakeStreamFactory} />);
  await user.click(await screen.findByRole('button', { name: 'SIGN OUT' }));
  expect(c.logout).toHaveBeenCalledOnce();
  expect(await screen.findByRole('heading', { level: 1, name: /queue while the cheater plays/i })).toBeInTheDocument();
});
