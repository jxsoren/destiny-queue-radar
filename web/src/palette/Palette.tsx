import { useEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent, type MouseEvent, type ReactNode } from 'react';
import type { Api } from '../api/client';
import type { Match, SearchResult } from '../api/types';
import { Emblem, emblemUrl } from '../board/Emblem';
import { displayName, splitName } from '../board/names';
import { useNow } from '../lib/useNow';
import { Dialog } from './Dialog';
import { needsReauth, paletteError } from './errors';
import { buildGroups, flatten, isBungieName, type Row } from './rows';
import { PromptGlyph } from '../ui/PromptBar';
import { RadarMark } from '../ui/RadarMark';

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
const rowId = (key: string) => `palette-${key.replace(/[^A-Za-z0-9_-]/g, '-')}`;

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

  const activeKey = index >= 0 ? rows[index].key : null;
  useEffect(() => {
    if (activeKey !== null) document.getElementById(rowId(activeKey))?.scrollIntoView?.({ block: 'nearest' });
  }, [index, activeKey]);

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
      <div className="band on-mist flex shrink-0 min-h-0 items-center gap-3 px-4 py-3">
        <RadarMark size={38} />
        <h2 id="palette-title" className="t-name text-shadow-strong text-[22px]">
          Track a player
        </h2>
      </div>
      <input
        ref={inputRef}
        type="text"
        role="combobox"
        aria-expanded="true"
        aria-controls="palette-list"
        aria-autocomplete="list"
        aria-activedescendant={index >= 0 ? rowId(rows[index].key) : undefined}
        aria-label="Filter your lobbies, or enter a Bungie Name"
        placeholder="Lobby filter or Name#1234"
        autoComplete="off"
        spellCheck={false}
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setError(null);
          setActive(0);
        }}
        onKeyDown={onKeyDown}
        className="mx-4 mb-1.5 mt-3.5 block shrink-0 w-[calc(100%-2rem)] border-2 border-white/70 bg-white/5 px-3 py-2.5 text-[15px] text-white placeholder:italic placeholder:text-white/55 focus-visible:border-white focus-visible:outline-none"
      />
      {error && (
        <p role="alert" className="t-status mx-4 mt-2 bg-[#5a1f1c]/90 px-3 py-2">
          {error}
        </p>
      )}
      <div role="status">
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
        {searching && <Note busy>SEARCHING BUNGIE…</Note>}
      </div>
      <p role="status" aria-live="polite" className="sr-only">
        {announcement}
      </p>
      <div id="palette-list" role="listbox" aria-label="Players" className="min-h-0 flex-1 overflow-y-auto">
        <Group title="YOUR LAST MATCH">{renderRows(groups.last)}</Group>
        {groups.older.length > 0 && <Group title="OLDER LOBBIES">{renderRows(groups.older)}</Group>}
        {groups.search.length > 0 && <Group title="BUNGIE SEARCH">{renderRows(groups.search)}</Group>}
      </div>
      <p aria-hidden="true" className="mt-3 flex shrink-0 justify-end gap-5 bg-prompt px-4 py-2 text-[13px] text-prompt-ink">
        <span>
          <PromptGlyph>↑↓</PromptGlyph>Move
        </span>
        <span>
          <PromptGlyph>↵</PromptGlyph>Track
        </span>
        <span>
          <PromptGlyph red>esc</PromptGlyph>Dismiss
        </span>
      </p>
    </Dialog>
  );
}

function Group({ title, children }: { title: string; children: ReactNode }) {
  const id = `palette-group-${title.toLowerCase().replace(/\W+/g, '-')}`;
  return (
    <div role="group" aria-labelledby={id} className="pb-2">
      <p
        id={id}
        role="presentation"
        className="t-head flex items-center gap-2.5 px-4 pb-1.5 pt-3.5 text-white/75 after:h-px after:flex-1 after:bg-white/20 after:content-['']"
      >
        {title}
      </p>
      {children}
    </div>
  );
}

function Note({ children, accent = false, busy = false }: { children: ReactNode; accent?: boolean; busy?: boolean }) {
  return (
    <p className={`t-status flex items-center gap-2 px-4 py-2 ${accent ? 'text-[#ff8a85]' : 'text-white/75'}`}>
      {busy && (
        <span aria-hidden="true" className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-white/25 border-t-white motion-reduce:animate-none" />
      )}
      {children}
    </p>
  );
}

interface RowViewProps {
  row: Row;
  active: boolean;
  adding: boolean;
  onAct: () => void;
  onHover: () => void;
}

/** A search banner shows its emblem art at the left; a scrim starts under the name and turns solid by 45%, matching the other rows' tone. */
function bannerStyle(path: string): CSSProperties | undefined {
  const url = emblemUrl(path);
  if (!url) return undefined;
  return {
    backgroundImage: `linear-gradient(90deg, rgba(46,47,46,0) 8%, rgba(46,47,46,0.65) 14%, var(--tip-row) 45%), url("${url}")`,
    backgroundSize: '100% 100%, auto 100%',
    backgroundRepeat: 'no-repeat',
    backgroundPosition: 'left center',
  };
}

function RowView({ row, active, adding, onAct, onHover }: RowViewProps) {
  const common = {
    id: rowId(row.key),
    role: 'option' as const,
    'aria-selected': active,
    'data-active': active,
    onMouseDown: (e: MouseEvent) => e.preventDefault(), // keep focus in the input
    onMouseEnter: onHover,
    onClick: onAct,
  };
  if (row.kind === 'lobby') {
    return (
      <div
        {...common}
        aria-label={`${row.summary}, ${row.expanded ? 'expanded' : 'collapsed'}`}
        className="frame t-status mx-3 mb-1 flex cursor-pointer items-center justify-between px-2 py-2 text-white/85"
      >
        <span>{row.summary}</span>
        <span aria-hidden="true" className="btn-round">
          {row.expanded ? '−' : '+'}
        </span>
      </div>
    );
  }
  const label = displayName(row);
  const { name, code } = splitName(label);
  // An explicit name keeps "Name#1234" in one piece (the nested code span would read "Name #1234").
  const accessibleName = row.isSelf ? `${label} · YOU` : row.watching ? `${label} · WATCHING` : label;
  const style = bannerStyle(row.emblemBackgroundPath);
  return (
    <div
      {...common}
      aria-label={accessibleName}
      aria-disabled={row.isSelf || row.watching}
      style={style}
      className={`frame mx-3 mb-1.5 flex h-[52px] cursor-pointer items-center gap-3 bg-black/20 pr-3 ${style ? 'pl-16' : 'pl-2'}`}
    >
      {!style && <Emblem key={row.emblemPath} path={row.emblemPath} size={36} />}
      <span className={`text-shadow-strong min-w-0 flex-1 truncate text-[15px] font-bold ${row.isSelf ? 'opacity-60' : ''}`}>
        {name}
        <span className="font-normal opacity-70">{code}</span>
      </span>
      {row.isSelf && <span className="t-status opacity-70">YOU</span>}
      {row.watching && !row.isSelf && <span className="t-status text-live">WATCHING</span>}
      {active && !row.isSelf && !row.watching && !adding && (
        <span aria-hidden="true" className="t-status">
          ↵ TRACK
        </span>
      )}
      {adding && <span className="t-status opacity-70">ADDING…</span>}
    </div>
  );
}
