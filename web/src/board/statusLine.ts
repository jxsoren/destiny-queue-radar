import type { Player, Status } from '../api/types';
import { formatAgo, formatTimer, since } from '../lib/format';

export interface StatusLine {
  text: string;
  /** Count-up timer for players in a match. */
  timer: string | null;
  /** Show the padlock (private profiles). */
  lock: boolean;
}

const up = (s: string | undefined) => (s ?? '').toUpperCase();

/** Where a player not in PvP is: ORBIT, the social space's name, or PVE · ACTIVITY. */
function placeLabel(s: Status): string {
  switch (s.place) {
    case 'ORBIT':
      return 'ORBIT';
    case 'SOCIAL':
      return up(s.activity) || 'SOCIAL';
    case 'PVE':
      return s.activity ? `PVE · ${up(s.activity)}` : 'PVE';
    default:
      return '—';
  }
}

function lastPvp(s: Status, now: number): string {
  const ago = since(s.lastPvpEndedAt, now);
  return ago === null ? '' : ` · LAST PVP ${formatAgo(ago)}`;
}

/** The one mono status line on a card (spec §3.3). */
export function statusLine(p: Player, now: number): StatusLine {
  const s = p.status;
  const plain = (text: string): StatusLine => ({ text, timer: null, lock: false });
  switch (s.state) {
    case 'IN_MATCH': {
      const text = [up(s.playlist || s.mode), up(s.activity)].filter(Boolean).join(' · ') || 'PVP';
      const elapsed = since(s.startedAt, now);
      return { text, timer: elapsed === null ? null : formatTimer(elapsed), lock: false };
    }
    case 'LIKELY_QUEUING': {
      const ago = since(s.lastPvpEndedAt, now);
      const what = up(s.lastPvpPlaylist) || 'PVP';
      return plain(`${placeLabel(s)} · ${what} ENDED ${ago === null ? 'RECENTLY' : `${formatAgo(ago)} AGO`}`);
    }
    case 'NOT_IN_PVP':
      return plain(placeLabel(s) + lastPvp(s, now));
    case 'OFFLINE':
      return plain('OFFLINE' + lastPvp(s, now));
    case 'PRIVATE':
      return { text: 'PRIVATE PROFILE', timer: null, lock: true };
    case 'NOT_FOUND':
      return plain('ACCOUNT NOT FOUND');
    default:
      return plain('CHECKING…');
  }
}
