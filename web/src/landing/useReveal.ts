import { useEffect, type CSSProperties, type RefObject } from 'react';

/**
 * Section entrances, once each. The hidden start state applies only under `data-motion="on"`,
 * which this sets after the observer exists and only when the visitor allows motion, so a page
 * whose script fails or whose visitor prefers reduced motion shows everything at rest.
 */
export function useReveal(root: RefObject<HTMLElement | null>) {
  useEffect(() => {
    const el = root.current;
    if (!el || typeof IntersectionObserver === 'undefined') return;
    if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return;

    const io = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (!e.isIntersecting) continue;
          e.target.classList.add('is-in');
          io.unobserve(e.target);
        }
      },
      { rootMargin: '0px 0px -10% 0px', threshold: 0.08 },
    );
    el.querySelectorAll('[data-reveal]').forEach((t) => io.observe(t));
    el.dataset.motion = 'on';
    return () => {
      io.disconnect();
      delete el.dataset.motion;
    };
  }, [root]);
}

/** The place of a part in its section's entrance: heading 0, text 1, picture 2 … */
export const order = (i: number) => ({ '--i': i }) as CSSProperties;
