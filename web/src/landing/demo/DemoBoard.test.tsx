import { act, render, screen, within } from '@testing-library/react';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { DemoBoard } from './DemoBoard';
import { DEMO_STEP_MS } from './script';

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

test('a complete board on first render, inside a data-ui fragment', () => {
  render(<DemoBoard />);
  const demo = screen.getByLabelText('Sample board with made-up players');
  expect(demo).toHaveAttribute('data-ui');
  expect(within(demo).getAllByRole('listitem')).toHaveLength(6);
  expect(within(demo).queryByRole('button', { name: /stop watching/i })).toBeNull();
});

test('players change columns on the script', () => {
  render(<DemoBoard />);
  const inMatch = () => screen.getByRole('region', { name: /^IN MATCH/ });
  expect(within(inMatch()).queryByTestId('card-demo-kestrel')).toBeNull();
  act(() => {
    vi.advanceTimersByTime(DEMO_STEP_MS);
  });
  expect(within(inMatch()).getByTestId('card-demo-kestrel')).toBeInTheDocument();
});

test('index.html has a no-script board with sign-in and the disclaimer', () => {
  const html = readFileSync(resolve(__dirname, '../../../index.html'), 'utf8');
  const noscript = html.slice(html.indexOf('<noscript>'), html.indexOf('</noscript>'));
  expect(noscript).toContain('href="/auth/login"');
  expect(noscript).toContain('Sign in with Bungie');
  expect(noscript).toContain('not affiliated');
  expect(noscript).toContain('IN MATCH');
});
