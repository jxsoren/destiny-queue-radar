import { useEffect, useMemo, useReducer, useRef, useState } from 'react';
import { BoardView } from '../../board/BoardView';
import { boardReducer, initialBoard } from '../../board/reducer';
import { useNow } from '../../lib/useNow';
import { createDemo, demoPlayers } from './script';

/** The landing page's live thing: the real board on a scripted stream (spec §7). */
export function DemoBoard() {
  const [state, dispatch] = useReducer(boardReducer, initialBoard, (s) =>
    boardReducer(s, { type: 'snapshot', snapshot: { players: demoPlayers(Date.now()), apiStatus: 'ok' } }),
  );
  const reduced = useMemo(() => !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches, []);
  const [paused, setPaused] = useState(false);
  const [onScreen, setOnScreen] = useState(true);
  const figure = useRef<HTMLDivElement>(null);
  const demo = useRef<ReturnType<typeof createDemo>>(undefined);

  useEffect(() => {
    if (reduced) return;
    const d = createDemo(dispatch);
    demo.current = d;
    return () => {
      d.pause();
      demo.current = undefined;
    };
  }, [reduced]);

  useEffect(() => {
    const el = figure.current;
    if (reduced || !el || typeof IntersectionObserver === 'undefined') return;
    const io = new IntersectionObserver((entries) => {
      const last = entries[entries.length - 1];
      if (last) setOnScreen(last.isIntersecting);
    });
    io.observe(el);
    return () => io.disconnect();
  }, [reduced]);

  const running = !paused && onScreen;
  useEffect(() => {
    if (running) demo.current?.play();
    else demo.current?.pause();
  }, [running, reduced]);

  const now = useNow(1000);
  const players = useMemo(() => [...state.players.values()], [state.players]);
  return (
    <div ref={figure}>
      {!reduced && (
        <div className="mb-2 flex justify-end">
          <button
            type="button"
            aria-pressed={paused}
            aria-label={paused ? 'Play sample board animation' : 'Pause sample board animation'}
            onClick={() => setPaused((p) => !p)}
            className="border-2 border-ink bg-ground px-3 py-1 font-mono text-xs uppercase tracking-wider text-ink hover:bg-ink hover:text-ground focus-visible:outline-2 focus-visible:outline-offset-2"
          >
            {paused ? 'PLAY' : 'PAUSE'}
          </button>
        </div>
      )}
      <div data-ui aria-label="Sample board with made-up players" role="figure" className="bg-ground text-ink">
        <BoardView players={players} now={now} />
      </div>
    </div>
  );
}
