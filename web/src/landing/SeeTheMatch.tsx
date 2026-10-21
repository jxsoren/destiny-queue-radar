import { DemoBoard } from './demo/DemoBoard';
import { order } from './useReveal';

/** See the match: the live demo board, full width (spec §5.2 item 2). */
export function SeeTheMatch() {
  return (
    <section id="board" aria-labelledby="board-title" className="landing-section on-mist">
      <h2 id="board-title" data-reveal style={order(0)} className="section-title">
        See the match
      </h2>
      <p data-reveal style={order(1)} className="section-sub">
        Live demo · sample players, made-up names
      </p>
      <p data-reveal style={order(1)} className="max-w-xl text-[15px]">
        This board is running now on six made-up players. Cards move as each one starts or finishes a match.
      </p>
      <div data-reveal="picture" style={order(2)} className="mt-6">
        <DemoBoard />
      </div>
      <div data-reveal style={order(3)}>
        <p className="mt-4 text-[14px]">
          Each player is checked about once a minute while your board is open. Refresh asks again sooner, at most every 30
          seconds.
        </p>
        <p className="mt-1 text-[12px] opacity-85">Sample players. The names are made up.</p>
      </div>
    </section>
  );
}
