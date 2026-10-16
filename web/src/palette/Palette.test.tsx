import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { ApiError } from '../api/client';
import { NOW, iso, makeMatch, makeParticipant } from '../test/fixtures';
import { Palette } from './Palette';

const lastMatch = makeMatch({
  instanceId: '9001',
  players: [
    makeParticipant({ membershipId: 'me', bungieName: 'Tester#0001', isSelf: true }),
    makeParticipant({ membershipId: 'v', bungieName: 'Vex#1111' }),
    makeParticipant({ membershipId: 'w1', bungieName: 'Watched#0002' }),
  ],
});
const olderMatch = makeMatch({
  instanceId: '9002',
  playlist: 'Control',
  activity: 'Endless Vale',
  period: iso(41 * 60_000),
  players: [makeParticipant({ membershipId: 'k', bungieName: 'Kestrel#0412' }), makeParticipant({ membershipId: 'me', bungieName: 'Tester#0001', isSelf: true })],
});

function setup(over: Partial<Record<'recentMatches' | 'search' | 'addWatch', ReturnType<typeof vi.fn>>> = {}) {
  const client = {
    recentMatches: vi.fn().mockResolvedValue([lastMatch, olderMatch]),
    search: vi.fn().mockResolvedValue({ membershipType: 3, membershipId: '9', bungieName: 'Ghost#0880', emblemPath: '/g.jpg' }),
    addWatch: vi.fn().mockResolvedValue(undefined),
    ...over,
  };
  const onClose = vi.fn();
  const onAdded = vi.fn();
  const watchedIds = new Set(['w1']);
  render(<Palette open onClose={onClose} client={client} watchedIds={watchedIds} onAdded={onAdded} />);
  return { client, onClose, onAdded, user: userEvent.setup() };
}

const input = () => screen.getByRole('combobox');

describe('Palette', () => {
  // The palette reads the real clock for lobby ages; pin Date (timers stay real) to the fixtures' NOW.
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(NOW);
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  test('renders nothing when closed', () => {
    const { container } = render(
      <Palette open={false} onClose={vi.fn()} client={{ recentMatches: vi.fn(), search: vi.fn(), addWatch: vi.fn() }} watchedIds={new Set()} onAdded={vi.fn()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  test('lists your last match, older lobbies collapsed, self and watched marked', async () => {
    setup();
    expect(await screen.findByRole('option', { name: /Vex#1111/ })).toBeInTheDocument();
    expect(input()).toHaveFocus();
    expect(screen.getByRole('option', { name: /Tester#0001/ })).toHaveTextContent('YOU');
    expect(screen.getByRole('option', { name: /Watched#0002/ })).toHaveTextContent('WATCHING');
    expect(screen.getByRole('option', { name: /CONTROL · ENDLESS VALE · 2 PLAYERS · 41M/ })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: /Kestrel/ })).toBeNull();
    expect(screen.getByRole('group', { name: 'YOUR LAST MATCH' })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'OLDER LOBBIES' })).toBeInTheDocument();
  });

  test('arrows move, Enter expands a lobby, Enter adds a player and keeps the palette open', async () => {
    const { client, onAdded, onClose, user } = setup();
    await screen.findByRole('option', { name: /Vex#1111/ });
    await user.keyboard('{ArrowDown}{ArrowDown}{ArrowDown}');
    const lobby = screen.getByRole('option', { name: /CONTROL/ });
    expect(lobby).toHaveAttribute('aria-selected', 'true');
    expect(input()).toHaveAttribute('aria-activedescendant', lobby.id);
    await user.keyboard('{Enter}');
    const kestrel = await screen.findByRole('option', { name: /Kestrel#0412/ });

    await user.keyboard('{ArrowDown}{Enter}');
    expect(client.addWatch).toHaveBeenCalledWith({ membershipType: 3, membershipId: 'k', source: 'match' });
    await vi.waitFor(() => expect(kestrel).toHaveTextContent('WATCHING'));
    expect(onAdded).toHaveBeenCalledOnce();
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  test('Enter on yourself or a watched player does nothing', async () => {
    const { client, user } = setup();
    await screen.findByRole('option', { name: /Vex#1111/ });
    await user.keyboard('{Enter}');
    await user.keyboard('{ArrowDown}{ArrowDown}{Enter}');
    expect(client.addWatch).not.toHaveBeenCalled();
  });

  test('typing filters lobbies by name', async () => {
    const { user } = setup();
    await screen.findByRole('option', { name: /Vex#1111/ });
    await user.type(input(), 'kes');
    expect(screen.getByRole('option', { name: /Kestrel#0412/ })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: /Vex#1111/ })).toBeNull();
  });

  test('a Bungie Name is searched after the debounce and can be added', async () => {
    const { client, user } = setup();
    await screen.findByRole('option', { name: /Vex#1111/ });
    await user.type(input(), 'Ghost#0880');
    const ghost = await screen.findByRole('option', { name: /Ghost#0880/ });
    expect(client.search).toHaveBeenCalledTimes(1);
    expect(client.search).toHaveBeenCalledWith('Ghost#0880');
    expect(within(screen.getByRole('group', { name: 'BUNGIE SEARCH' })).getByRole('option')).toBe(ghost);
    expect(ghost.querySelector('img')).toHaveAttribute('src', 'https://www.bungie.net/g.jpg');
    await user.keyboard('{Enter}');
    expect(client.addWatch).toHaveBeenCalledWith({ membershipType: 3, membershipId: '9', source: 'search' });
  });

  test.each([
    [404, null, 'PLAYER NOT FOUND'],
    [409, null, 'WATCHLIST FULL (50)'],
    [429, 12, 'SLOW DOWN — RETRY IN 12S'],
    [503, null, 'BUNGIE UNAVAILABLE'],
    [502, null, 'COULD NOT REACH BUNGIE'],
  ])('add error %i shows "%s" inline', async (status, retryAfter, message) => {
    const { user } = setup({ addWatch: vi.fn().mockRejectedValue(new ApiError(status, 'x', retryAfter, false)) });
    await user.click(await screen.findByRole('option', { name: /Vex#1111/ }));
    expect(await screen.findByRole('alert')).toHaveTextContent(message);
    expect(screen.getByRole('option', { name: /Vex#1111/ })).not.toHaveTextContent('WATCHING');
  });

  test('a search miss shows PLAYER NOT FOUND', async () => {
    const { user } = setup({ search: vi.fn().mockRejectedValue(new ApiError(404, 'no player', null, false)) });
    await screen.findByRole('option', { name: /Vex#1111/ });
    await user.type(input(), 'Nobody#0001');
    expect(await screen.findByRole('alert')).toHaveTextContent('PLAYER NOT FOUND');
  });

  test('a reauth 401 on recent matches offers sign-in', async () => {
    setup({ recentMatches: vi.fn().mockRejectedValue(new ApiError(401, 'sign in again', null, true)) });
    expect(await screen.findByText(/SIGN IN AGAIN TO SEE YOUR MATCHES/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'SIGN IN' })).toHaveAttribute('href', '/auth/login');
  });

  test('Esc closes', async () => {
    const { onClose, user } = setup();
    await screen.findByRole('option', { name: /Vex#1111/ });
    await user.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalledOnce();
  });
});
