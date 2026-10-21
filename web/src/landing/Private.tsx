import { order } from './useReveal';

const FACTS: readonly [label: string, value: string][] = [
  ['Watchlist', 'Up to 50 players'],
  ['Who can see it', 'Only you'],
  ['Public flags', 'None'],
];

const QUESTIONS: readonly [question: string, answer: string][] = [
  [
    'Does Queue Radar see my Bungie password?',
    'No. You sign in on Bungie.net and Bungie gives Queue Radar a key to your account’s data. Your password never reaches us.',
  ],
  [
    'What does signing in let it read?',
    'Your Bungie Name and your recent matches, so the players from your last lobby are one keypress away.',
  ],
  [
    'Is “likely queuing” Bungie’s matchmaking queue?',
    'No. Bungie’s API doesn’t show who is queuing. It’s a guess from timing: back in orbit within five minutes of a PvP match.',
  ],
  [
    'How fresh is a status?',
    'Each player is checked about once a minute while your board is open, and for five minutes after you close it. Refresh asks again, at most every 30 seconds. Anything older than three minutes is marked stale.',
  ],
  ['Will the player know I’m watching?', 'No. Nothing is sent to them and nothing about them is published.'],
  ['How many players can I watch?', '50. Remove one in Manage mode to make room.'],
  ['What if Bungie’s API is down?', 'A warning strip on your board says so, and statuses stay frozen until it’s back.'],
  ['Is Queue Radar made by Bungie?', 'No. It’s independent, and not affiliated with or endorsed by Bungie.'],
];

/** Private by design, laid out like the inspect screen's Material details (spec §5.2 item 4). */
export function Private() {
  return (
    <section id="privacy" aria-labelledby="privacy-title" className="landing-section on-mist">
      <h2 id="privacy-title" data-reveal style={order(0)} className="section-title">
        Private by design
      </h2>
      <p data-reveal style={order(0)} className="section-sub">
        Material details
      </p>
      <div className="about">
        <div>
          <dl data-reveal style={order(1)} className="about-facts">
            {FACTS.map(([label, value]) => (
              <div key={label}>
                <dt>{label}</dt>
                <dd>{value}</dd>
              </div>
            ))}
          </dl>
          <p data-reveal style={order(1)} className="about-paragraph">
            Your list is yours. Nobody else can see who you watch, and Rook#2093 is never told, contacted or flagged. There
            are no shared lists and no public pages about anyone. A private profile stays private: it shows as Private, and
            we never work around it through the player’s fireteam.
          </p>
        </div>
        <div>
          <h2 id="questions" className="sr-only">
            Questions
          </h2>
          <dl className="about-questions">
            {QUESTIONS.map(([q, a], i) => (
              <div key={q} data-reveal style={order(1 + (i % 2))} className="tip">
                <dt className="tip-title normal-case">{q}</dt>
                <dd className="tip-text">{a}</dd>
              </div>
            ))}
          </dl>
        </div>
      </div>
    </section>
  );
}
