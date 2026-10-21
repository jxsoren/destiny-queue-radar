import { act, fireEvent, render, screen, within } from '@testing-library/react';
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

function setup(over: Partial<Record<'recentMatches' | 'search' | 'addWatch', ReturnType<typeof vi.fn>>> = {}, fakeTimers = false) {
  const client = {
    recentMatches: vi.fn().mockResolvedValue([lastMatch, olderMatch]),
    search: vi.fn().mockResolvedValue({ membershipType: 3, membershipId: '9', bungieName: 'Ghost#0880', emblemPath: '/g.jpg', emblemBackgroundPath: '/g_bg.jpg' }),
    addWatch: vi.fn().mockResolvedValue(undefined),
    ...over,
  };
  const onClose = vi.fn();
  const onAdded = vi.fn();
  const watchedIds = new Set(['w1']);
  render(<Palette open onClose={onClose} client={client} watchedIds={watchedIds} onAdded={onAdded} />);
  return { client, onClose, onAdded, user: userEvent.setup(fakeTimers ? { advanceTimers: (ms) => void vi.advanceTimersByTime(ms) } : undefined) };
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

describe('Palette accessibility and behaviour', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(NOW);
    Element.prototype.scrollIntoView = vi.fn();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  test('the active option and the input have visible indicators', async () => {
    setup();
    const vex = await screen.findByRole('option', { name: /Vex#1111/ });
    const me = screen.getByRole('option', { name: /Tester#0001/ });
    expect(me).toHaveClass('outline', 'outline-2', '-outline-offset-2', 'outline-ink', 'text-ink');
    expect(vex).not.toHaveClass('outline-ink');
    expect(input().className).not.toContain('focus:outline-none');
    expect(input()).toHaveClass('focus-visible:border-b-2', 'focus-visible:border-ink');
  });

  test('the active option is scrolled into view', async () => {
    const { user } = setup();
    await screen.findByRole('option', { name: /Vex#1111/ });
    (Element.prototype.scrollIntoView as ReturnType<typeof vi.fn>).mockClear();
    await user.keyboard('{ArrowDown}');
    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith({ block: 'nearest' });
  });

  test('only options live inside the listbox; notes sit in a status region', async () => {
    setup({ recentMatches: vi.fn().mockRejectedValue(new ApiError(401, 'sign in again', null, true)) });
    const note = await screen.findByText(/SIGN IN AGAIN TO SEE YOUR MATCHES/);
    expect(note.closest('[role=listbox]')).toBeNull();
    expect(note.closest('[role=status]')).not.toBeNull();
    const listbox = screen.getByRole('listbox');
    expect(listbox.querySelectorAll('a, [role=status]')).toHaveLength(0);
    expect(document.getElementById('palette-group-your-last-match')).toHaveAttribute('role', 'presentation');
  });

  test('lobby options say whether they are expanded', async () => {
    const { user } = setup();
    const lobby = await screen.findByRole('option', { name: /CONTROL.*, collapsed/ });
    await user.click(lobby);
    expect(await screen.findByRole('option', { name: /CONTROL.*, expanded/ })).toBeInTheDocument();
  });

  test('a successful add is announced politely', async () => {
    const { user } = setup();
    await user.click(await screen.findByRole('option', { name: /Vex#1111/ }));
    const said = await screen.findByText('WATCHING VEX#1111');
    expect(said.closest('[role=status]')).toHaveAttribute('aria-live', 'polite');
  });

  test('the notes status region stays in the DOM, and visible, when empty', async () => {
    setup();
    await screen.findByRole('option', { name: /Vex#1111/ });
    const region = screen.getAllByRole('status').find((el) => !el.classList.contains('sr-only'))!;
    expect(region).toBeEmptyDOMElement();
    expect(region.className).not.toContain('empty:hidden');
  });

  test('the active row is not re-scrolled when rows change but the active row does not', async () => {
    const scroll = vi.fn();
    Element.prototype.scrollIntoView = scroll;
    let finish!: () => void;
    const { user } = setup({ addWatch: vi.fn().mockReturnValue(new Promise<void>((r) => (finish = r))) });
    await user.click(await screen.findByRole('option', { name: /Vex#1111/ }));
    const calls = scroll.mock.calls.length;
    await act(async () => finish());
    await screen.findByRole('option', { name: /Vex#1111 · WATCHING/ });
    expect(scroll.mock.calls.length).toBe(calls);
  });

  test('typing clears the previous error', async () => {
    const { user } = setup({ addWatch: vi.fn().mockRejectedValue(new ApiError(409, 'x', null, false)) });
    await user.click(await screen.findByRole('option', { name: /Vex#1111/ }));
    expect(await screen.findByRole('alert')).toBeInTheDocument();
    await user.type(input(), 'v');
    expect(screen.queryByRole('alert')).toBeNull();
  });

  test('a double Enter sends one POST', async () => {
    const { client } = setup({ addWatch: vi.fn().mockReturnValue(new Promise(() => {})) });
    await screen.findByRole('option', { name: /Vex#1111/ });
    fireEvent.keyDown(input(), { key: 'ArrowDown' });
    act(() => {
      fireEvent.keyDown(input(), { key: 'Enter' });
      fireEvent.keyDown(input(), { key: 'Enter' });
    });
    expect(client.addWatch).toHaveBeenCalledTimes(1);
  });
});

describe('Palette search debounce', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    Element.prototype.scrollIntoView = vi.fn();
    // Testing Library's asyncWrapper only advances fake timers when it sees a `jest` global.
    Object.assign(globalThis, { jest: { advanceTimersByTime: (ms: number) => vi.advanceTimersByTime(ms) } });
  });
  afterEach(() => {
    delete (globalThis as { jest?: unknown }).jest;
    vi.useRealTimers();
  });

  test('every keystroke restarts the 300ms timer; one search with the final name', async () => {
    const { client, user } = setup({}, true);
    await act(() => vi.advanceTimersByTimeAsync(0));
    await user.type(input(), 'Ghost#0880');
    await act(() => vi.advanceTimersByTimeAsync(200));
    await user.type(input(), '{Backspace}1'); // still a valid Bungie name: Ghost#0881
    await act(() => vi.advanceTimersByTimeAsync(299));
    expect(client.search).not.toHaveBeenCalled();
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(client.search).toHaveBeenCalledTimes(1);
    expect(client.search).toHaveBeenCalledWith('Ghost#0881');
  });
});
