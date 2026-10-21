import { fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test } from 'vitest';
import { Landing } from './Landing';

test('the primary action is Sign in with Bungie', () => {
  render(<Landing />);
  const links = screen.getAllByRole('link', { name: /sign in with bungie/i });
  expect(links.length).toBeGreaterThan(0);
  for (const link of links) expect(link).toHaveAttribute('href', '/auth/login');
});

test('says it is not affiliated with Bungie, in the footer', () => {
  render(<Landing />);
  const footer = screen.getByRole('contentinfo');
  expect(within(footer).getByText(/not affiliated with or endorsed by Bungie/i)).toBeInTheDocument();
});

test('carries the live demo board', () => {
  render(<Landing />);
  expect(screen.getByLabelText('Sample board with made-up players')).toHaveAttribute('data-ui');
});

test('one h1, and every heading fits the two-line budget', () => {
  render(<Landing />);
  expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
  for (const h of screen.getAllByRole('heading')) {
    expect((h.textContent ?? '').length, h.textContent ?? '').toBeLessThanOrEqual(40);
  }
});

test('the first sign-in link is in the first section', () => {
  render(<Landing />);
  const first = document.querySelector('main > section, main > header');
  expect(first).not.toBeNull();
  expect(first!.querySelector('a[href="/auth/login"]')).not.toBeNull();
});

test('the made-up names are labelled as made up and never linked', () => {
  render(<Landing />);
  expect(screen.getByText('Sample players. The names are made up.')).toBeInTheDocument();
  for (const link of screen.getAllByRole('link')) expect(link.textContent ?? '').not.toMatch(/#\d{4}/);
});

test('the first screen is the item: name, type line, quoted flavour and the pitch', () => {
  render(<Landing />);
  expect(screen.getByRole('heading', { level: 1, name: 'Queue Radar' })).toBeInTheDocument();
  expect(screen.getByText('Crucible Reconnaissance')).toBeInTheDocument();
  expect(screen.getByText('“You are not hidden.”')).toBeInTheDocument();
  expect(screen.getAllByText('See when the players you follow are in a PvP match.').length).toBeGreaterThan(0);
  expect(screen.getByRole('figure', { name: 'Sample players in a match and queuing' })).toBeInTheDocument();
});

test('no cheater framing anywhere on the page', () => {
  const { container } = render(<Landing />);
  expect(container.textContent ?? '').not.toMatch(/cheat|lost to|beat you|suspect/i);
});

test('how it works: the slider steps through the four stops', () => {
  render(<Landing />);
  const how = document.getElementById('how')!;
  const slider = within(how).getByRole('slider', { name: 'Steps for tracking a player' });
  const clause = () => within(how).getByRole('heading', { level: 2 }).textContent;
  const fragment = () => within(how).getByTestId('how-fragment');

  expect(slider).toHaveValue('0');
  expect(slider).toHaveAttribute('aria-valuetext', 'Match ends');
  expect(clause()).toBe('First, the match ends');
  expect(fragment()).toHaveTextContent('Press / to track a player.');

  fireEvent.change(slider, { target: { value: '1' } });
  expect(clause()).toBe('Then press /');
  expect(fragment()).toHaveTextContent('YOUR LAST MATCH');
  expect(fragment()).toHaveTextContent('Rook#2093');

  fireEvent.change(slider, { target: { value: '2' } });
  expect(clause()).toBe('Pick the name you lost to');
  expect(fragment()).toHaveTextContent('ENTER TO WATCH');
  expect(fragment()).not.toHaveTextContent('CHECKING…');

  fireEvent.change(slider, { target: { value: '3' } });
  expect(slider).toHaveAttribute('aria-valuetext', 'Watching');
  expect(clause()).toBe('Now you’re watching them');
  expect(fragment()).toHaveTextContent('WATCHING');
  expect(fragment()).toHaveTextContent('CHECKING…');
});

test('how it works: the stop names move the slider too', async () => {
  const user = userEvent.setup();
  render(<Landing />);
  const how = document.getElementById('how')!;
  await user.click(within(how).getByRole('button', { name: 'Pick the name' }));
  expect(within(how).getByRole('slider')).toHaveValue('2');
});

test('the moment the match ends: Rook goes from In match to Queuing?', () => {
  render(<Landing />);
  const columns = document.getElementById('columns')!;
  const moment = within(columns).getByRole('figure', { name: /Rook#2093 at the end of his match/ });
  expect(moment).toHaveAttribute('data-ui');
  expect(moment).toHaveTextContent('TRIALS · BURNOUT');
  expect(moment).toHaveTextContent('09:48');
  expect(moment).toHaveTextContent('ORBIT · TRIALS ENDED 1M AGO');
  expect(within(columns).getAllByRole('row')).toHaveLength(9);
});
