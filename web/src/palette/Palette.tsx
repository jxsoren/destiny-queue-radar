import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type MouseEvent, type ReactNode } from 'react';
import type { Api } from '../api/client';
import type { Match, SearchResult } from '../api/types';
import { Emblem } from '../board/Emblem';
import { displayName, splitName } from '../board/names';
import { useNow } from '../lib/useNow';
import { Dialog } from './Dialog';
import { needsReauth, paletteError } from './errors';
import { buildGroups, flatten, isBungieName, type Row } from './rows';

export const SEARCH_DEBOUNCE_MS = 300;

export interface PaletteProps {
  open: boolean;
  onClose: () => void;
  client: Pick<Api, 'recentMatches' | 'search' | 'addWatch'>;
  watchedIds: ReadonlySet<string>;
  /** Called after a successful add, so the board refetches the watchlist. */
  onAdded: () => void;
}

/** The "track a player" command palette (spec §4). Each opening starts fresh. */
export function Palette(props: PaletteProps) {
  return props.open ? <PaletteBody {...props} /> : null;
}

type Recent = { kind: 'loading' } | { kind: 'ready'; matches: Match[] } | { kind: 'error'; error: unknown };

const NO_MATCHES: Match[] = [];
const rowId = (r: Row) => `palette-${r.key.replace(/[^A-Za-z0-9_-]/g, '-')}`;

function toggled(set: ReadonlySet<string>, id: string): Set<string> {
  const next = new Set(set);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  return next;
}

function PaletteBody({ onClose, client, watchedIds, onAdded }: PaletteProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const now = useNow(60_000);
  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);
  const [recent, setRecent] = useState<Recent>({ kind: 'loading' });
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(new Set());
  const [added, setAdded] = useState<ReadonlySet<string>>(new Set());
  const [search, setSearch] = useState<SearchResult | null>(null);
  const [searching, setSearching] = useState(false);
  const [adding, setAdding] = useState<string | null>(null);
  const addingRef = useRef(false);
  const [announcement, setAnnouncement] = useState('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    client.recentMatches().then(
      (matches) => live && setRecent({ kind: 'ready', matches }),
      (err: unknown) => live && setRecent({ kind: 'error', error: err }),
    );
    return () => {
      live = false;
    };
  }, [client]);

  useEffect(() => {
    const q = query.trim();
    setSearch(null);
    if (!isBungieName(q)) {
      setSearching(false);
      return;
    }
    setSearching(true);
    let live = true;
    const timer = setTimeout(() => {
      client
        .search(q)
        .then(
          (result) => {
            if (!live) return;
            setSearch(result);
            setError(null);
          },
          (err: unknown) => live && setError(paletteError(err, 'search')),
        )
        .finally(() => live && setSearching(false));
    }, SEARCH_DEBOUNCE_MS);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [query, client]);

  const matches = recent.kind === 'ready' ? recent.matches : NO_MATCHES;
  const watched = useMemo(() => new Set([...watchedIds, ...added]), [watchedIds, added]);
  const groups = useMemo(() => buildGroups({ matches, expanded, query, search, watched, now }), [matches, expanded, query, search, watched, now]);
  const rows = useMemo(() => flatten(groups), [groups]);
  const index = rows.length === 0 ? -1 : Math.min(active, rows.length - 1);

  useEffect(() => {
    if (index >= 0) document.getElementById(rowId(rows[index]))?.scrollIntoView?.({ block: 'nearest' });
  }, [index, rows]);

  async function act(row: Row) {
    if (row.kind === 'lobby') {
      setExpanded((s) => toggled(s, row.instanceId));
      return;
    }
    if (row.isSelf || row.watching || addingRef.current) return;
    addingRef.current = true;
    setAdding(row.membershipId);
    setError(null);
    try {
      await client.addWatch({ membershipType: row.membershipType, membershipId: row.membershipId, source: row.source });
      setAdded((s) => new Set(s).add(row.membershipId));
      setAnnouncement(`WATCHING ${displayName(row).toUpperCase()}`);
      onAdded();
    } catch (err) {
      setError(paletteError(err, 'add'));
    } finally {
      addingRef.current = false;
      setAdding(null);
    }
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
      const row = rows[index];
      if (row) void act(row);
    }
  }

  const renderRows = (list: Row[]) =>
    list.map((row) => {
      const i = rows.indexOf(row);
      return (
        <RowView
          key={row.key}
          row={row}
          active={i === index}
          adding={row.kind === 'player' && adding === row.membershipId}
          onAct={() => {
            setActive(i);
            void act(row);
          }}
          onHover={() => setActive(i)}
        />
      );
    });

  return (
    <Dialog onClose={onClose} labelledBy="palette-title" initialFocus={inputRef}>
      <h2 id="palette-title" className="sr-only">
        Track a player
      </h2>
      <input
        ref={inputRef}
        type="text"
        role="combobox"
        aria-expanded="true"
        aria-controls="palette-list"
        aria-autocomplete="list"
        aria-activedescendant={index >= 0 ? rowId(rows[index]) : undefined}
        aria-label="Filter your lobbies, or enter a Bungie Name"
        placeholder="FILTER LOBBIES OR ENTER NAME#1234"
        autoComplete="off"
        spellCheck={false}
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setError(null);
          setActive(0);
        }}
        onKeyDown={onKeyDown}
        className="tele w-full bg-ground px-4 py-4 text-[13px] text-ink placeholder:text-dim border-b-2 border-transparent focus-visible:border-b-2 focus-visible:border-ink focus-visible:outline-none"
      />
      {error && (
        <p role="alert" className="tele bg-raised px-4 py-2 text-accent">
          {error}
        </p>
      )}
      <div role="status" className="empty:hidden">
        {recent.kind === 'loading' && <Note>LOADING YOUR MATCHES…</Note>}
        {recent.kind === 'error' &&
          (needsReauth(recent.error) ? (
            <Note accent>
              SIGN IN AGAIN TO SEE YOUR MATCHES ·{' '}
              <a href="/auth/login" className="underline">
                SIGN IN
              </a>
            </Note>
          ) : (
            <Note accent>{paletteError(recent.error, 'matches')}</Note>
          ))}
        {recent.kind === 'ready' && matches.length === 0 && <Note>NO RECENT PVP MATCHES</Note>}
        {searching && <Note>SEARCHING BUNGIE…</Note>}
      </div>
      <p role="status" aria-live="polite" className="sr-only">
        {announcement}
      </p>
      <div id="palette-list" role="listbox" aria-label="Players" className="max-h-[60vh] overflow-y-auto bg-ground">
        <Group title="YOUR LAST MATCH">{renderRows(groups.last)}</Group>
        {groups.older.length > 0 && <Group title="OLDER LOBBIES">{renderRows(groups.older)}</Group>}
        {groups.search.length > 0 && <Group title="BUNGIE SEARCH">{renderRows(groups.search)}</Group>}
      </div>
    </Dialog>
  );
}

function Group({ title, children }: { title: string; children: ReactNode }) {
  const id = `palette-group-${title.toLowerCase().replace(/\W+/g, '-')}`;
  return (
    <div role="group" aria-labelledby={id} className="pb-2">
      <p id={id} role="presentation" className="tele px-4 pb-1 pt-3 text-dim">
        {title}
      </p>
      {children}
    </div>
  );
}

function Note({ children, accent = false }: { children: ReactNode; accent?: boolean }) {
  return <p className={`tele px-4 py-2 ${accent ? 'text-accent' : 'text-dim'}`}>{children}</p>;
}

interface RowViewProps {
  row: Row;
  active: boolean;
  adding: boolean;
  onAct: () => void;
  onHover: () => void;
}

function RowView({ row, active, adding, onAct, onHover }: RowViewProps) {
  const base = `flex cursor-pointer items-center gap-3 px-4 py-2 ${active ? 'bg-raised text-ink outline outline-2 -outline-offset-2 outline-ink' : ''}`;
  const common = {
    id: rowId(row),
    role: 'option' as const,
    'aria-selected': active,
    onMouseDown: (e: MouseEvent) => e.preventDefault(), // keep focus in the input
    onMouseEnter: onHover,
    onClick: onAct,
  };
  if (row.kind === 'lobby') {
    return (
      <div {...common} aria-label={`${row.summary}, ${row.expanded ? 'expanded' : 'collapsed'}`} className={`tele justify-between ${base} ${active ? 'text-ink' : 'text-dim'}`}>
        <span>{row.summary}</span>
        <span aria-hidden="true">{row.expanded ? '−' : '+'}</span>
      </div>
    );
  }
  const label = displayName(row);
  const { name, code } = splitName(label);
  // An explicit name keeps "Name#1234" in one piece (the nested code span would read "Name #1234").
  const accessibleName = row.isSelf ? `${label} · YOU` : row.watching ? `${label} · WATCHING` : label;
  return (
    <div {...common} aria-label={accessibleName} aria-disabled={row.isSelf || row.watching} className={base}>
      <Emblem key={row.emblemPath} path={row.emblemPath} size={28} />
      <span className={`min-w-0 flex-1 truncate ${row.isSelf ? 'text-dim' : 'text-ink'}`}>
        {name}
        <span className="text-dim">{code}</span>
      </span>
      {row.isSelf && <span className="tele text-dim">YOU</span>}
      {row.watching && !row.isSelf && <span className="tele text-accent">WATCHING</span>}
      {adding && <span className="tele text-dim">ADDING…</span>}
    </div>
  );
}
