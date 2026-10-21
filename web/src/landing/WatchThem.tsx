import { useState, type CSSProperties } from 'react';
import { PlayerCard } from '../board/PlayerCard';
import { EmptyBoard } from '../board/TrackSlot';
import { CheckGlyph } from '../ui/glyphs';
import { LobbyStill } from './fragments/Lobby';
import { ROOK_CHECKING, ROOK_ID, SAMPLE_NOW } from './fragments/sample';
import { order } from './useReveal';

const STOPS = [
  { name: 'Match ends', clause: 'First, the match ends' },
  { name: 'Press /', clause: 'Then press /' },
  { name: 'Pick a name', clause: 'Pick a name' },
  { name: 'Watching', clause: 'Now you’re watching them' },
] as const;

const NOBODY: ReadonlySet<string> = new Set();
const ROOK: ReadonlySet<string> = new Set([ROOK_ID]);

/** The product at each stop: the empty board, the palette, Rook picked, Rook watched. */
function Still({ stop, onStop }: { stop: number; onStop: (n: number) => void }) {
  if (stop === 0) return <EmptyBoard onOpen={() => onStop(1)} />;
  if (stop === 1) return <LobbyStill activeId="sample-wren" watched={NOBODY} />;
  if (stop === 2) return <LobbyStill activeId={ROOK_ID} watched={NOBODY} />;
  return (
    <>
      <LobbyStill activeId={ROOK_ID} watched={ROOK} />
      <ul className="mt-3 grid gap-2">
        <PlayerCard player={ROOK_CHECKING} now={SAMPLE_NOW} managing={false} />
      </ul>
    </>
  );
}

/** Completed objectives are solid green with a check (spec §1); the current step is blue and ringed. */
function nodeClass(i: number, stop: number): string {
  if (i < stop) return 'perk-done';
  if (i === stop) return 'perk-active perk-selected';
  return 'perk-upcoming';
}

/** How it works: one slider drives the clause, the step track and the fragment (spec §5.2 item 1). */
export function WatchThem() {
  const [stop, setStop] = useState(0);
  const current = STOPS[stop];
  const progress = { '--progress': stop / (STOPS.length - 1) } as CSSProperties;
  return (
    <section id="how" aria-labelledby="how-clause" className="landing-section on-mist">
      <p data-reveal style={order(0)} className="section-title">
        How it works
      </p>
      <p data-reveal style={order(0)} className="section-sub">
        Four steps
      </p>
      <div className="how">
        <div>
          <h2 id="how-clause" data-reveal style={order(0)} className="how-clause">
            {current.clause}
          </h2>
          <p data-reveal style={order(1)} className="how-note">
            Not in your last match? Type their Bungie Name, like Rook#2093, and pick them from Bungie’s search.
          </p>
          <div data-reveal style={{ ...progress, ...order(3) }} className="how-control">
            <input
              type="range"
              min={0}
              max={STOPS.length - 1}
              step={1}
              value={stop}
              aria-label="Steps for tracking a player"
              aria-valuetext={current.name}
              onChange={(e) => setStop(Number(e.target.value))}
              className="how-range"
            />
            <ol className="how-stops">
              {STOPS.map((s, i) => (
                <li key={s.name}>
                  <button type="button" aria-pressed={i === stop} onClick={() => setStop(i)} className="how-stop">
                    <span aria-hidden="true" className={`perk how-node ${nodeClass(i, stop)}`}>
                      {i < stop ? <CheckGlyph /> : i + 1}
                    </span>
                    <span>{s.name}</span>
                    {i < stop && <span className="sr-only"> (done)</span>}
                  </button>
                </li>
              ))}
            </ol>
          </div>
        </div>
        <div data-reveal="picture" style={order(2)} className="how-stage">
          <div data-ui data-testid="how-fragment">
            <div key={stop} className="how-still">
              <Still stop={stop} onStop={setStop} />
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
