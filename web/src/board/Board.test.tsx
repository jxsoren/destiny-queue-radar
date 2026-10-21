import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, test, vi } from 'vitest';
import type { Api } from '../api/client';
import { FakeStream, fakeStreamFactory } from '../test/fakeStream';
import { makeMatch, makePlayer } from '../test/fixtures';
import { Board } from './Board';

const me = { id: 1, bungieName: 'Tester#0001', membershipType: 3, membershipId: 'me' };

function fakeClient() {
  return {
    me: vi.fn().mockResolvedValue(me),
    watchlist: vi.fn().mockResolvedValue([]),
    addWatch: vi.fn().mockResolvedValue(undefined),
    removeWatch: vi.fn().mockResolvedValue(undefined),
    search: vi.fn(),
    recentMatches: vi.fn().mockResolvedValue([makeMatch()]),
    refresh: vi.fn().mockResolvedValue(undefined),
    logout: vi.fn().mockResolvedValue(undefined),
  };
}

function setup() {
  const client = fakeClient();
  const onSignedOut = vi.fn();
  render(<Board me={me} onSignedOut={onSignedOut} client={client as unknown as Api} stream={fakeStreamFactory} />);
  return { client, onSignedOut, user: userEvent.setup() };
}

const snapshot = (players = [makePlayer()]) =>
  act(() => FakeStream.latest().emit('snapshot', { players, apiStatus: 'ok' }));

beforeEach(() => FakeStream.reset());

describe('Board', () => {
  test('shows connecting, then the board from the snapshot', () => {
    setup();
    expect(screen.getByText('CONNECTING…')).toBeInTheDocument();
    snapshot();
    expect(within(screen.getByRole('region', { name: /^NOT IN PVP/ })).getByTestId('card-4611')).toBeInTheDocument();
    expect(screen.getByTestId('watchlist')).toHaveTextContent('1 / 50');
    expect(screen.getByRole('button', { name: /TRACK A PLAYER/ })).toBeInTheDocument();
  });

  test('an empty board invites "/"', () => {
    setup();
    snapshot([]);
    expect(screen.getByText('Press / to track a player.')).toBeInTheDocument();
  });

  test('the prompt bar names the / shortcut and offers + Track on phones', async () => {
    const { user } = setup();
    snapshot();
    const bar = screen.getByTestId('prompt-bar');
    expect(within(bar).getByText('Track a player')).toBeInTheDocument();
    await user.click(within(bar).getByRole('button', { name: '+ Track' }));
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  test('"/" and the slot open the palette; adding refetches the watchlist', async () => {
    const { client, user } = setup();
    snapshot([]);
    await user.keyboard('/');
    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    await user.click(await screen.findByRole('option', { name: /Vex#1111/ }));
    await vi.waitFor(() => expect(client.watchlist).toHaveBeenCalled());
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).toBeNull();
    await user.click(screen.getByRole('button', { name: /TRACK A PLAYER/ }));
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  test('manage mode: × hides the card, undo brings it back without a delete', async () => {
    const { client, user } = setup();
    snapshot();
    await user.click(screen.getByRole('button', { name: 'MANAGE' }));
    expect(screen.queryByRole('button', { name: /TRACK A PLAYER/ })).toBeNull();
    await user.click(screen.getByRole('button', { name: 'Stop watching Vex#1111' }));
    expect(screen.queryByTestId('card-4611')).toBeNull();
    expect(screen.getByText('STOPPED WATCHING VEX#1111 ·')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'UNDO' })).toHaveFocus();
    expect(screen.getByTestId('board-root')).toHaveClass('pb-40');
    await user.click(screen.getByRole('button', { name: 'UNDO' }));
    expect(screen.getByTestId('card-4611')).toBeInTheDocument();
    expect(client.removeWatch).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'Stop watching Vex#1111' })).toHaveFocus();
  });

  test('focus returns to "+ Track a player" when the undo bar commits outside manage mode', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const { client, user } = setup();
      snapshot();
      await user.click(screen.getByRole('button', { name: 'MANAGE' }));
      await user.click(screen.getByRole('button', { name: 'Stop watching Vex#1111' }));
      await user.keyboard('{Escape}'); // leave manage mode; focus stays on UNDO
      expect(screen.getByRole('button', { name: 'UNDO' })).toHaveFocus();
      await act(() => vi.advanceTimersByTimeAsync(5_000));
      expect(client.removeWatch).toHaveBeenCalled();
      expect(screen.queryByRole('button', { name: 'UNDO' })).toBeNull();
      expect(screen.getByRole('button', { name: /TRACK A PLAYER/ })).toHaveFocus();
    } finally {
      vi.useRealTimers();
    }
  });

  test('focus falls back to MANAGE/DONE when the last card is removed in manage mode', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const { user } = setup();
      snapshot();
      await user.click(screen.getByRole('button', { name: 'MANAGE' }));
      await user.click(screen.getByRole('button', { name: 'Stop watching Vex#1111' }));
      await act(() => vi.advanceTimersByTimeAsync(5_000));
      expect(screen.queryByRole('button', { name: 'UNDO' })).toBeNull();
      expect(screen.getByRole('button', { name: 'DONE' })).toHaveFocus();
    } finally {
      vi.useRealTimers();
    }
  });

  test('pb-40 also applies while a removal error is showing', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const { client, user } = setup();
      client.removeWatch.mockRejectedValue(new Error('x'));
      snapshot();
      await user.click(screen.getByRole('button', { name: 'MANAGE' }));
      await user.click(screen.getByRole('button', { name: 'Stop watching Vex#1111' }));
      await act(() => vi.advanceTimersByTimeAsync(5_000));
      expect(screen.queryByRole('button', { name: 'UNDO' })).toBeNull();
      expect(screen.getByRole('alert')).toHaveTextContent('COULD NOT STOP WATCHING VEX#1111');
      expect(screen.getByTestId('board-root')).toHaveClass('pb-40');
    } finally {
      vi.useRealTimers();
    }
  });

  test('Esc leaves manage mode', async () => {
    const { user } = setup();
    snapshot();
    await user.click(screen.getByRole('button', { name: 'MANAGE' }));
    await user.keyboard('{Escape}');
    expect(screen.getByRole('button', { name: 'MANAGE' })).toBeInTheDocument();
  });

  test('refresh starts the cooldown; sign out logs out and leaves', async () => {
    const { client, onSignedOut, user } = setup();
    snapshot();
    await user.click(screen.getByRole('button', { name: 'REFRESH' }));
    expect(client.refresh).toHaveBeenCalledOnce();
    expect(await screen.findByRole('button', { name: /REFRESH \d+S/ })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: 'SIGN OUT' }));
    expect(client.logout).toHaveBeenCalledOnce();
    expect(onSignedOut).toHaveBeenCalledOnce();
  });

  test('the Bungie banner follows api_status', () => {
    setup();
    snapshot();
    act(() => FakeStream.latest().emit('api_status', { bungie: 'down' }));
    expect(screen.getByRole('alert')).toHaveTextContent('BUNGIE API UNAVAILABLE — STATUSES FROZEN');
  });
});
