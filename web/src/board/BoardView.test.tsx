import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test } from 'vitest';
import { NOW, iso, makePlayer } from '../test/fixtures';
import { BoardView } from './BoardView';

const players = [
  makePlayer({ membershipId: 'm1', bungieName: 'Rook#2093', status: { state: 'IN_MATCH', startedAt: iso(60_000) } }),
  makePlayer({ membershipId: 'q1', bungieName: 'Mire#3307', status: { state: 'LIKELY_QUEUING', lastPvpEndedAt: iso(60_000) } }),
  makePlayer({ membershipId: 'i1', bungieName: 'Vex#1111' }),
  makePlayer({ membershipId: 'o1', bungieName: 'Tallow#5510', status: { state: 'OFFLINE' } }),
  makePlayer({ membershipId: 'o2', bungieName: 'Kestrel#0412', status: { state: 'NOT_FOUND' } }),
];

test('each player sits in its column, with counts', () => {
  render(<BoardView players={players} now={NOW} />);
  expect(within(screen.getByRole('region', { name: /^IN MATCH/ })).getByTestId('card-m1')).toBeInTheDocument();
  expect(within(screen.getByRole('region', { name: /^QUEUING/ })).getByTestId('card-q1')).toBeInTheDocument();
  expect(within(screen.getByRole('region', { name: /^NOT IN PVP/ })).getByTestId('card-i1')).toBeInTheDocument();
  const offline = screen.getByRole('region', { name: /^OFFLINE/ });
  expect(within(offline).getAllByRole('listitem')).toHaveLength(2);
  expect(offline).toHaveAccessibleName('OFFLINE 02');
});

test('the stats block: in-match bar is green while anyone is in a match, and the watchlist row', () => {
  render(<BoardView players={players} now={NOW} />);
  expect(screen.getByTestId('bar-match')).toHaveClass('bg-live');
  expect(screen.getByTestId('bar-queue')).toHaveClass('bg-white');
  expect(screen.getByTestId('count-offline')).toHaveTextContent('2');
  expect(screen.getByTestId('watchlist')).toHaveTextContent('Watchlist 5 / 50');
});

test('on phones a stat bar filters the columns; a second tap or All resets', async () => {
  const user = userEvent.setup();
  render(<BoardView players={players} now={NOW} />);
  const region = (name: RegExp) => screen.getByRole('region', { name });
  expect(region(/^IN MATCH/)).not.toHaveClass('hidden');
  expect(region(/^OFFLINE/)).not.toHaveClass('hidden');

  await user.click(screen.getByRole('button', { name: 'Offline 2' }));
  expect(screen.getByRole('button', { name: 'Offline 2' })).toHaveAttribute('aria-pressed', 'true');
  expect(region(/^OFFLINE/)).not.toHaveClass('hidden');
  expect(region(/^IN MATCH/)).toHaveClass('hidden');

  await user.click(screen.getByRole('button', { name: 'Offline 2' }));
  expect(region(/^IN MATCH/)).not.toHaveClass('hidden');

  await user.click(screen.getByRole('button', { name: 'In match 1' }));
  expect(region(/^OFFLINE/)).toHaveClass('hidden');
  await user.click(screen.getByRole('button', { name: 'All' }));
  expect(screen.getByRole('button', { name: 'All' })).toHaveAttribute('aria-pressed', 'true');
  expect(region(/^OFFLINE/)).not.toHaveClass('hidden');
});

test('renders the slot it is given', () => {
  render(<BoardView players={[]} now={NOW} slot={<p>slot here</p>} />);
  expect(screen.getByText('slot here')).toBeInTheDocument();
});

test('an empty column says so', () => {
  render(<BoardView players={players.filter((p) => p.membershipId !== 'm1')} now={NOW} />);
  expect(within(screen.getByRole('region', { name: /^IN MATCH/ })).getByText('No players here.')).toBeInTheDocument();
});

test('the watchlist line shows the server size when given', () => {
  render(<BoardView players={players} now={NOW} watching={12} />);
  expect(screen.getByTestId('watchlist')).toHaveTextContent('Watchlist 12 / 50');
});

test('the big number and the watchlist row use the same count', () => {
  const { container } = render(<BoardView players={players} now={NOW} watching={12} />);
  expect(container.querySelector('.t-big data')).toHaveTextContent('12');
  expect(screen.getByTestId('watchlist')).toHaveTextContent('Watchlist 12 / 50');
});
