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

const heroLobby = () => screen.getByRole('group', { name: 'Sample lobby with made-up players' });

test('hero lobby: Rook#2093 is picked, and Enter watches him', async () => {
  const user = userEvent.setup();
  render(<Landing />);
  const lobby = heroLobby();
  expect(lobby).toHaveAttribute('data-ui');
  expect(within(lobby).getAllByRole('option')).toHaveLength(6);
  const rook = within(lobby).getByRole('option', { name: 'Rook#2093' });
  expect(rook).toHaveAttribute('aria-selected', 'true');
  expect(rook).toHaveTextContent('ENTER TO WATCH');

  await user.click(within(lobby).getByRole('combobox'));
  await user.keyboard('{Enter}');
  const watching = within(lobby).getByRole('option', { name: 'Rook#2093 · WATCHING' });
  expect(watching).toHaveTextContent('WATCHING');
  expect(watching).not.toHaveTextContent('ENTER TO WATCH');
});

test('hero lobby: hovering moves the pick, clicking watches, and you cannot watch yourself', async () => {
  const user = userEvent.setup();
  render(<Landing />);
  const lobby = heroLobby();
  const pike = within(lobby).getByRole('option', { name: 'Pike#0937' });
  await user.hover(pike);
  expect(pike).toHaveAttribute('aria-selected', 'true');
  expect(pike).toHaveTextContent('ENTER TO WATCH');
  expect(within(lobby).getByRole('option', { name: 'Rook#2093' })).not.toHaveTextContent('ENTER TO WATCH');

  await user.click(within(lobby).getByRole('option', { name: 'Wren#4420 · YOU' }));
  expect(within(lobby).queryByRole('option', { name: /Wren#4420 · WATCHING/ })).toBeNull();

  await user.click(within(lobby).getByRole('option', { name: 'Mire#3307' }));
  expect(within(lobby).getByRole('option', { name: 'Mire#3307 · WATCHING' })).toBeInTheDocument();
});

test('hero lobby: typing filters the six names', async () => {
  const user = userEvent.setup();
  render(<Landing />);
  const lobby = heroLobby();
  await user.type(within(lobby).getByRole('combobox'), 'ro');
  expect(within(lobby).getAllByRole('option').map((o) => o.getAttribute('aria-label'))).toEqual(['Rook#2093']);
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
  expect(fragment()).toHaveTextContent('Press / to track the player who just beat you.');

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
