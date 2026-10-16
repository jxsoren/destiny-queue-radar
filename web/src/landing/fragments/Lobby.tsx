import { useId, useMemo, useState, type KeyboardEvent, type ReactNode } from 'react';
import { Emblem } from '../../board/Emblem';
import { splitName } from '../../board/names';
import type { PlayerRow } from '../../palette/rows';
import { LOBBY_LABEL, lobbyRows, ROOK_ID } from './sample';

type Hint = { text: string; accent: boolean } | null;

/** The palette's hint for a row: you, already watched, or the one Enter would add. */
function hintFor(row: PlayerRow, active: boolean): Hint {
  if (row.isSelf) return { text: 'YOU', accent: false };
  if (row.watching) return { text: 'WATCHING', accent: true };
  return active ? { text: 'ENTER TO WATCH', accent: false } : null;
}

function accessibleName(row: PlayerRow): string {
  if (row.isSelf) return `${row.bungieName} · YOU`;
  return row.watching ? `${row.bungieName} · WATCHING` : row.bungieName;
}

const rowClass = (active: boolean) =>
  `lobby-row flex items-center gap-3 px-4 py-2 ${active ? 'bg-raised outline outline-2 -outline-offset-2 outline-ink' : ''}`;

/** One row as the palette draws it: emblem, name with the dimmed #code, and the hint. */
function RowContent({ row, active }: { row: PlayerRow; active: boolean }) {
  const { name, code } = splitName(row.bungieName);
  const hint = hintFor(row, active);
  return (
    <>
      <Emblem path={row.emblemPath} size={28} />
      <span className={`min-w-0 flex-1 truncate ${row.isSelf ? 'text-dim' : 'text-ink'}`}>
        {name}
        <span className="text-dim">{code}</span>
      </span>
      {hint && <span className={`tele shrink-0 ${hint.accent ? 'text-accent' : 'text-dim'}`}>{hint.text}</span>}
    </>
  );
}

function GroupLabel({ id, children }: { id?: string; children: ReactNode }) {
  return (
    <p id={id} className="tele px-4 pb-1 pt-3 text-dim">
      {children}
    </p>
  );
}

const ROOK_INDEX = lobbyRows('', new Set()).findIndex((r) => r.membershipId === ROOK_ID);

/**
 * The hero's working fragment: the palette's "Your last match" group with Rook#2093 picked.
 * Arrow keys, hover and typing move the pick as in the real palette; Enter or a click watches.
 */
export function LobbyFragment() {
  const base = useId();
  const listId = `${base}-list`;
  const labelId = `${base}-label`;
  const optionId = (membershipId: string) => `${base}-${membershipId}`;
  const [query, setQuery] = useState('');
  const [watched, setWatched] = useState<ReadonlySet<string>>(new Set());
  const [active, setActive] = useState(ROOK_INDEX);
  const [announcement, setAnnouncement] = useState('');
  const rows = useMemo(() => lobbyRows(query, watched), [query, watched]);
  const index = rows.length === 0 ? -1 : Math.min(active, rows.length - 1);

  function watch(row: PlayerRow) {
    if (row.isSelf || row.watching) return;
    setWatched((s) => new Set(s).add(row.membershipId));
    setAnnouncement(`WATCHING ${row.bungieName.toUpperCase()}`);
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActive(Math.min(index + 1, rows.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive(Math.max(index - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (index >= 0) watch(rows[index]);
    }
  }

  return (
    <div data-ui role="group" aria-label="Sample lobby with made-up players" className="lobby bg-ground text-ink">
      <input
        type="text"
        role="combobox"
        aria-expanded="true"
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={index >= 0 ? optionId(rows[index].membershipId) : undefined}
        aria-label="Search a Bungie Name"
        placeholder="Search a Bungie Name, like Rook#2093"
        autoComplete="off"
        spellCheck={false}
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setActive(0);
        }}
        onKeyDown={onKeyDown}
        className="lobby-input tele w-full bg-ground px-4 py-4 text-[13px] text-ink placeholder:text-dim"
      />
      <GroupLabel id={labelId}>{LOBBY_LABEL}</GroupLabel>
      <div id={listId} role="listbox" aria-labelledby={labelId} className="pb-2">
        {rows.map((row, i) => (
          <div
            key={row.key}
            id={optionId(row.membershipId)}
            role="option"
            aria-selected={i === index}
            aria-disabled={row.isSelf || row.watching}
            aria-label={accessibleName(row)}
            onMouseDown={(e) => e.preventDefault()}
            onMouseEnter={() => setActive(i)}
            onClick={() => {
              setActive(i);
              watch(row);
            }}
            className={`${rowClass(i === index)} cursor-pointer`}
          >
            <RowContent row={row} active={i === index} />
          </div>
        ))}
      </div>
      <p role="status" aria-live="polite" className="sr-only">
        {announcement}
      </p>
    </div>
  );
}

/** A still of the palette for the how-it-works slider: which row is picked, who is watched. */
export function LobbyStill({ activeId, watched }: { activeId: string | null; watched: ReadonlySet<string> }) {
  const rows = lobbyRows('', watched);
  return (
    <div className="lobby bg-ground text-ink">
      <p aria-hidden="true" className="lobby-input tele px-4 py-4 text-[13px] text-dim">
        FILTER LOBBIES OR ENTER NAME#1234
      </p>
      <GroupLabel>{LOBBY_LABEL}</GroupLabel>
      <ul className="pb-2">
        {rows.map((row) => {
          const active = row.membershipId === activeId;
          return (
            <li key={row.key} className={rowClass(active)}>
              <RowContent row={row} active={active} />
            </li>
          );
        })}
      </ul>
    </div>
  );
}
