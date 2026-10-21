import { PlayerCard } from '../board/PlayerCard';
import { Arrow } from './Arrow';
import { order } from './useReveal';
import { ROOK_IN_MATCH, ROOK_QUEUING, SAMPLE_NOW } from './fragments/sample';

const COLUMNS = ['What Queue Radar sees', 'How sure', 'What to do'] as const;

const STATES: readonly [state: string, sees: string, sure: string, todo: string][] = [
  ['In match', 'In a PvP match, with the playlist, the map and the time since it started', 'Reported by Bungie', 'Queue now. A match lasts about ten minutes.'],
  ['Likely queuing', 'Back in orbit or a social space within five minutes of a PvP match', 'Our guess from timing', 'Wait a few minutes, or queue a different playlist.'],
  ['Not in PvP', 'In the Tower, in orbit for a while, or in a PvE activity', 'Reported by Bungie', 'Queue. Look again before the next one.'],
  ['Offline', 'Not playing Destiny 2', 'Reported by Bungie', 'Queue.'],
  ['Private profile', 'Their privacy settings hide what they’re doing', 'Nothing to go on', 'No reading. We never guess from their fireteam.'],
  ['Account not found', 'The Bungie Name no longer finds an account', 'Nothing to go on', 'Check the name. Manage mode can check it again.'],
  ['Checking…', 'Just added; the first check hasn’t finished', 'Not yet', 'Give it up to a minute.'],
  ['Stale', 'The last good answer is more than three minutes old', 'Out of date', 'Treat the column as a hint until it updates.'],
];

/** The queue window (spec §5.2 item 3): Rook's card as his match ends, then what every state means. */
export function QueueWhileBusy() {
  return (
    <section id="columns" aria-labelledby="columns-title" className="landing-section on-mist">
      <h2 id="columns-title" data-reveal style={order(0)} className="section-title">
        The queue window
      </h2>
      <p data-reveal style={order(0)} className="section-sub">
        Rook#2093, end of a match
      </p>
      <p data-reveal style={order(1)} className="statement">
        “Likely queuing” is our guess, not Bungie’s queue. Bungie doesn’t say who is matchmaking, so a player back in orbit
        within five minutes of a PvP match shows here until the five minutes run out.
      </p>
      <div
        data-ui
        data-reveal="moment"
        style={order(2)}
        role="figure"
        aria-label="Rook#2093 at the end of his match, before and after"
        className="moment"
      >
        <div className="moment-col tip">
          <p className="tip-title flex justify-between">
            Before <span className="t-head text-live-text">IN MATCH</span>
          </p>
          <ul className="grid gap-2 p-3">
            <PlayerCard player={ROOK_IN_MATCH} now={SAMPLE_NOW} managing={false} />
          </ul>
          <p className="tip-text pt-0">Still in a match. A match lasts about ten minutes.</p>
        </div>
        <span className="moment-arrow">
          <Arrow size={24} />
        </span>
        <div className="moment-col moment-after tip">
          <p className="tip-title flex justify-between">
            After <span className="t-head">QUEUING</span>
          </p>
          <ul className="grid gap-2 p-3">
            <PlayerCard player={ROOK_QUEUING} now={SAMPLE_NOW} managing={false} />
          </ul>
          <p className="tip-text pt-0">Back in orbit. They may be queuing again.</p>
        </div>
      </div>
      <table data-reveal="picture" style={order(3)} className="states">
        <thead>
          <tr>
            <th scope="col">
              <span className="sr-only">State</span>
            </th>
            {COLUMNS.map((c) => (
              <th key={c} scope="col">
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {STATES.map(([state, ...answers], row) => (
            <tr key={state} data-reveal="row" style={order(row)}>
              <th scope="row">{state}</th>
              {answers.map((a, i) => (
                <td key={COLUMNS[i]} data-label={COLUMNS[i]}>
                  {a}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
