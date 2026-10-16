import { render } from '@testing-library/react';
import { afterEach, expect, test, vi } from 'vitest';
import { Landing } from './Landing';

type Callback = (entries: Partial<IntersectionObserverEntry>[]) => void;

/** A stand-in observer that records what it watches and lets the test say what entered. */
function fakeObserver() {
  const watched = new Set<Element>();
  let fire: Callback = () => {};
  class FakeIO {
    constructor(cb: Callback) {
      fire = cb;
    }
    observe(el: Element) {
      watched.add(el);
    }
    unobserve(el: Element) {
      watched.delete(el);
    }
    disconnect() {
      watched.clear();
    }
  }
  vi.stubGlobal('IntersectionObserver', FakeIO);
  return { watched, enter: (el: Element) => fire([{ target: el, isIntersecting: true }]) };
}

function stubReducedMotion(reduce: boolean) {
  vi.stubGlobal('matchMedia', (q: string) => ({ matches: reduce && q.includes('reduce'), media: q }));
}

afterEach(() => vi.unstubAllGlobals());

const root = () => document.querySelector('.landing') as HTMLElement;

test('without an observer the page never arms its entrances, so nothing can stay hidden', () => {
  vi.stubGlobal('IntersectionObserver', undefined);
  render(<Landing />);
  expect(root().dataset.motion).toBeUndefined();
  expect(document.querySelectorAll('[data-reveal]').length).toBeGreaterThan(8);
});

test('under reduced motion the entrances are not armed', () => {
  fakeObserver();
  stubReducedMotion(true);
  render(<Landing />);
  expect(root().dataset.motion).toBeUndefined();
});

test('each part is revealed once, when it enters, and then left alone', () => {
  const io = fakeObserver();
  stubReducedMotion(false);
  render(<Landing />);
  expect(root().dataset.motion).toBe('on');
  const heading = document.getElementById('board-title')!;
  expect(io.watched.has(heading)).toBe(true);
  expect(heading).not.toHaveClass('is-in');

  io.enter(heading);
  expect(heading).toHaveClass('is-in');
  expect(io.watched.has(heading)).toBe(false);
});

test('the parts of a section enter in reading order: heading, text, picture', () => {
  stubReducedMotion(false);
  fakeObserver();
  render(<Landing />);
  const board = document.getElementById('board')!;
  const order = [...board.querySelectorAll<HTMLElement>('[data-reveal]')].map((el) => el.style.getPropertyValue('--i'));
  expect(order).toEqual(['0', '1', '2', '3']);
});
