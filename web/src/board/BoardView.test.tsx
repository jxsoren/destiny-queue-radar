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
  expect(within(screen.getByRole('region', { name: /^LIKELY QUEUING/ })).getByTestId('card-q1')).toBeInTheDocument();
  expect(within(screen.getByRole('region', { name: /^NOT IN PVP/ })).getByTestId('card-i1')).toBeInTheDocument();
  const offline = screen.getByRole('region', { name: /^OFFLINE/ });
  expect(within(offline).getAllByRole('listitem')).toHaveLength(2);
  expect(offline).toHaveAccessibleName('OFFLINE 02');
});

test('the in-match count turns red when above zero', () => {
  render(<BoardView players={players} now={NOW} />);
  const desktop = screen.getByTestId('counts');
  expect(within(desktop).getByTestId('count-match')).toHaveClass('text-accent');
  expect(within(desktop).getByTestId('count-queue')).not.toHaveClass('text-accent');
});

test('phone tabs pick the visible column', async () => {
  const user = userEvent.setup();
  render(<BoardView players={players} now={NOW} />);
  expect(screen.getByRole('region', { name: /^IN MATCH/ })).not.toHaveClass('hidden');
  expect(screen.getByRole('region', { name: /^OFFLINE/ })).toHaveClass('hidden');
  await user.click(screen.getByRole('button', { name: /OFFLINE/ }));
  expect(screen.getByRole('button', { name: /OFFLINE/ })).toHaveAttribute('aria-pressed', 'true');
  expect(screen.getByRole('button', { name: /OFFLINE/ })).toHaveClass('border-ink');
  expect(screen.getByRole('button', { name: /IN MATCH/ })).not.toHaveClass('border-ink');
  expect(screen.getByRole('region', { name: /^OFFLINE/ })).not.toHaveClass('hidden');
  expect(screen.getByRole('region', { name: /^IN MATCH/ })).toHaveClass('hidden');
});

test('renders the slot it is given', () => {
  render(<BoardView players={[]} now={NOW} slot={<p>slot here</p>} />);
  expect(screen.getByText('slot here')).toBeInTheDocument();
});
