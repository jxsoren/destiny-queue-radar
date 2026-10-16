import { DemoBoard } from './demo/DemoBoard';

/** Claim 3, "See when they're in a match": the live demo board, full width. */
export function SeeTheMatch() {
  return (
    <section id="board" aria-labelledby="board-title" className="showcase">
      <div className="showcase-head">
        <h2 id="board-title" className="section-title">
          See when they’re in a match
        </h2>
        <p className="showcase-sentence">
          This board is running now on six made-up players. Cards move as each one starts or finishes a match.
        </p>
      </div>
      <div className="showcase-board">
        <DemoBoard />
      </div>
      <div className="showcase-foot">
        <p className="showcase-line">
          Each player is checked about once a minute while your board is open. Refresh asks again sooner, at most every 30
          seconds.
        </p>
        <p className="small-note">Sample players. The names are made up.</p>
      </div>
    </section>
  );
}
