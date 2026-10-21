import { PITCH } from '../ui/copy';

const NAV: readonly [label: string, href: string][] = [
  ['How it works', '#how'],
  ['Board', '#board'],
  ['Queue window', '#columns'],
  ['Privacy', '#privacy'],
  ['Questions', '#questions'],
];

/** The footer (spec §5.2 item 6): the pitch, page links and the legal lines. */
export function Footer() {
  return (
    <footer className="site-foot on-mist">
      <div>
        <p className="foot-lead">{PITCH}</p>
        <nav aria-label="On this page" className="foot-nav">
          <ul>
            {NAV.map(([label, href]) => (
              <li key={href}>
                <a href={href} className="hover:underline">
                  {label}
                </a>
              </li>
            ))}
          </ul>
        </nav>
      </div>
      <div className="foot-legal grid gap-1">
        <p>Queue Radar is not affiliated with or endorsed by Bungie. Destiny 2 is a trademark of Bungie, Inc.</p>
        <p>Player data comes from the Bungie.net API. Emblems on this page are drawn for it.</p>
        <p>Signing in shows Queue Radar your Bungie Name and recent matches. Never your password.</p>
      </div>
    </footer>
  );
}
