import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { DemoBoard } from './DemoBoard';
import { DEMO_STEP_MS } from './script';

beforeEach(() => vi.useFakeTimers());
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

const inMatch = () => screen.getByRole('region', { name: /^IN MATCH/ });

function stubIO() {
  let fire: (e: Partial<IntersectionObserverEntry>[]) => void = () => {};
  class FakeIO {
    constructor(cb: typeof fire) {
      fire = cb;
    }
    observe() {}
    unobserve() {}
    disconnect() {}
  }
  vi.stubGlobal('IntersectionObserver', FakeIO);
  return (isIntersecting: boolean) => act(() => fire([{ isIntersecting }]));
}

test('reduced motion: the board stays at rest and has no pause control', () => {
  vi.stubGlobal('matchMedia', (q: string) => ({ matches: q.includes('reduce'), media: q }));
  render(<DemoBoard />);
  act(() => {
    vi.advanceTimersByTime(DEMO_STEP_MS * 3);
  });
  expect(within(inMatch()).queryByTestId('card-demo-kestrel')).toBeNull();
  expect(within(inMatch()).getAllByRole('listitem')).toHaveLength(1);
  expect(screen.queryByRole('button', { name: /demo/i })).toBeNull();
});

test('pauses while off screen and resumes when visible again', () => {
  const setVisible = stubIO();
  render(<DemoBoard />);
  setVisible(false);
  act(() => {
    vi.advanceTimersByTime(DEMO_STEP_MS * 2);
  });
  expect(within(inMatch()).queryByTestId('card-demo-kestrel')).toBeNull();
  setVisible(true);
  act(() => {
    vi.advanceTimersByTime(DEMO_STEP_MS);
  });
  expect(within(inMatch()).getByTestId('card-demo-kestrel')).toBeInTheDocument();
});

test('a pause button stops and restarts the demo', () => {
  render(<DemoBoard />);
  const btn = screen.getByRole('button', { name: 'Pause sample board animation' });
  expect(btn).toHaveTextContent('PAUSE');
  expect(btn).toHaveAttribute('aria-pressed', 'false');
  fireEvent.click(btn);
  expect(btn).toHaveTextContent('PLAY');
  expect(btn).toHaveAttribute('aria-pressed', 'true');
  act(() => {
    vi.advanceTimersByTime(DEMO_STEP_MS * 3);
  });
  expect(within(inMatch()).queryByTestId('card-demo-kestrel')).toBeNull();
  fireEvent.click(btn);
  act(() => {
    vi.advanceTimersByTime(DEMO_STEP_MS);
  });
  expect(within(inMatch()).getByTestId('card-demo-kestrel')).toBeInTheDocument();
});

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
