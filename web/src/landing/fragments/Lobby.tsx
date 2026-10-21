import { Emblem } from '../../board/Emblem';
import { splitName } from '../../board/names';
import type { PlayerRow } from '../../palette/rows';
import { LOBBY_LABEL, lobbyRows } from './sample';

type Hint = { text: string; live: boolean } | null;

/** The palette's hint for a row: you, already watched, or the one Enter would add. */
function hintFor(row: PlayerRow, active: boolean): Hint {
  if (row.isSelf) return { text: 'YOU', live: false };
  if (row.watching) return { text: 'WATCHING', live: true };
  return active ? { text: 'ENTER TO WATCH', live: false } : null;
}

/** One row as the palette draws it: emblem, name with the dimmed #code, and the hint. */
function RowContent({ row, active }: { row: PlayerRow; active: boolean }) {
  const { name, code } = splitName(row.bungieName);
  const hint = hintFor(row, active);
  return (
    <>
      <Emblem path={row.emblemPath} size={28} />
      <span className={`min-w-0 flex-1 truncate text-[13px] font-bold ${row.isSelf ? 'opacity-60' : ''}`}>
        {name}
        <span className="font-normal opacity-70">{code}</span>
      </span>
      {hint && <span className={`t-status shrink-0 ${hint.live ? 'text-live' : 'opacity-75'}`}>{hint.text}</span>}
    </>
  );
}

/** A still of the palette for the how-it-works slider: which row is picked, who is watched. */
export function LobbyStill({ activeId, watched }: { activeId: string | null; watched: ReadonlySet<string> }) {
  const rows = lobbyRows('', watched);
  return (
    <div className="lobby">
      <p className="band on-mist t-name flex min-h-0 items-center px-3 py-2 text-base">Track a player</p>
      <p aria-hidden="true" className="lobby-input">
        Lobby filter or Name#1234
      </p>
      <p className="t-head px-3 pb-1.5 pt-2.5 text-white/75">{LOBBY_LABEL}</p>
      <ul className="pb-2">
        {rows.map((row) => {
          const active = row.membershipId === activeId;
          return (
            <li key={row.key} data-active={active} className="lobby-row frame">
              <RowContent row={row} active={active} />
            </li>
          );
        })}
      </ul>
      <p aria-hidden="true" className="bg-prompt px-3 py-1.5 text-right text-[12px] text-prompt-ink">
        ↑↓ Move · ↵ Track · esc Dismiss
      </p>
    </div>
  );
}
