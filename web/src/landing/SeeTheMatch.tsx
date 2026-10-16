import { DemoBoard } from './demo/DemoBoard';
import { order } from './useReveal';

/** Claim 3, "See when they're in a match": the live demo board, full width. */
export function SeeTheMatch() {
  return (
    <section id="board" aria-labelledby="board-title" className="showcase">
      <div className="showcase-head">
        <h2 id="board-title" data-reveal style={order(0)} className="section-title">
          See when they’re in a match
        </h2>
        <p data-reveal style={order(1)} className="showcase-sentence">
          This board is running now on six made-up players. Cards move as each one starts or finishes a match.
        </p>
      </div>
      <div data-reveal="picture" style={order(2)} className="showcase-board">
        <DemoBoard />
      </div>
      <div data-reveal style={order(3)} className="showcase-foot">
        <p className="showcase-line">
          Each player is checked about once a minute while your board is open. Refresh asks again sooner, at most every 30
          seconds.
        </p>
        <p className="small-note">Sample players. The names are made up.</p>
      </div>
    </section>
  );
}
