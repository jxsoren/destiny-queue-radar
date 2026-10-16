import { useState, type CSSProperties } from 'react';
import { PlayerCard } from '../board/PlayerCard';
import { EmptyBoard } from '../board/TrackSlot';
import { LobbyStill } from './fragments/Lobby';
import { ROOK_CHECKING, ROOK_ID, SAMPLE_NOW } from './fragments/sample';
import { order } from './useReveal';

const STOPS = [
  { name: 'Match ends', clause: 'First, the match ends' },
  { name: 'Press /', clause: 'Then press /' },
  { name: 'Pick the name', clause: 'Pick the name you lost to' },
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
      <ul className="how-card grid gap-px bg-line">
        <PlayerCard player={ROOK_CHECKING} now={SAMPLE_NOW} managing={false} />
      </ul>
    </>
  );
}

/** Claim 2, "Watch them": one slider drives the clause and the fragment through four stops. */
export function WatchThem() {
  const [stop, setStop] = useState(0);
  const current = STOPS[stop];
  const progress = { '--progress': stop / (STOPS.length - 1) } as CSSProperties;
  return (
    <section id="how" aria-labelledby="how-clause" className="how">
      <div className="how-words">
        <h2 id="how-clause" data-reveal style={order(0)} className="how-clause">
          {current.clause}
        </h2>
        <p data-reveal style={order(1)} className="how-note">
          Not in your last match? Type their Bungie Name, like Rook#2093, and pick them from Bungie’s search.
        </p>
      </div>
      <div data-reveal="picture" style={order(2)} className="how-stage">
        <div data-ui data-testid="how-fragment" className="how-fragment bg-ground text-ink">
          <div key={stop} className="how-still">
            <Still stop={stop} onStop={setStop} />
          </div>
        </div>
      </div>
      <div data-reveal style={{ ...progress, ...order(3) }} className="how-control">
        <div className="how-track">
          <span aria-hidden="true" className="how-track-fill" />
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
        </div>
        <ol className="how-stops">
          {STOPS.map((s, i) => (
            <li key={s.name}>
              <button type="button" aria-pressed={i === stop} onClick={() => setStop(i)} className="how-stop">
                {s.name}
              </button>
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}
