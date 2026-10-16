import { Arrow } from './Arrow';
import { order } from './useReveal';

const NAV: readonly [label: string, href: string][] = [
  ['How it works', '#how'],
  ['Board', '#board'],
  ['Columns', '#columns'],
  ['Privacy', '#privacy'],
  ['Questions', '#questions'],
];

/** A footer of tiles: the last two rows of the board, closing on the name and the action. */
export function Footer() {
  return (
    <footer className="tiles">
      <div data-reveal="tile" style={order(0)} className="tile tile-statement">
        <p className="tile-lead">Watch the players you suspect, and queue while they’re busy.</p>
        <div className="tile-legal">
          <p>Queue Radar is not affiliated with or endorsed by Bungie. Destiny 2 is a trademark of Bungie, Inc.</p>
          <p>Player data comes from the Bungie.net API. Emblems on this page are drawn for it.</p>
        </div>
      </div>
      <nav aria-label="On this page" data-reveal="tile" style={order(1)} className="tile tile-nav">
        <ul>
          {NAV.map(([label, href]) => (
            <li key={href}>
              <a href={href}>{label}</a>
            </li>
          ))}
        </ul>
      </nav>
      <div data-reveal="tile" style={order(2)} className="tile tile-key">
        <p className="tile-key-glyph" aria-hidden="true">
          /
        </p>
        <p>
          <span className="sr-only">The / key </span>opens the tracker anywhere on your board.
        </p>
      </div>
      <div data-reveal="tile" style={order(3)} className="tile tile-facts">
        <p>Signing in shows Queue Radar your Bungie Name and recent matches. Never your password.</p>
      </div>
      <div data-reveal="tile" style={order(0)} className="tile tile-wordmark">
        <p>Queue Radar</p>
      </div>
      <a href="/auth/login" data-reveal="tile" style={order(1)} className="tile tile-action">
        <span>Sign in with Bungie</span>
        <Arrow size={88} stroke={4} />
      </a>
    </footer>
  );
}
