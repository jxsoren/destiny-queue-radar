import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { api as defaultApi, type Api } from '../api/client';
import type { Me } from '../api/types';
import { useNow } from '../lib/useNow';
import type { StreamFactory } from '../live/stream';
import { useBoard } from '../live/useBoard';
import { useRefresh } from '../live/useRefresh';
import { UndoBar } from '../manage/UndoBar';
import { useRemoval } from '../manage/useRemoval';
import { Palette } from '../palette/Palette';
import { usePaletteHotkeys } from '../palette/useHotkeys';
import { Announcer } from './Announcer';
import { ApiBanner, StreamNotice } from './Banners';
import { BoardView } from './BoardView';
import { Header } from './Header';
import { displayName } from './names';
import { latestSuccess } from './stale';
import { EmptyBoard, TrackSlot } from './TrackSlot';
import { Backdrop } from '../ui/Backdrop';
import { PromptBar, PromptGlyph } from '../ui/PromptBar';

export interface BoardProps {
  me: Me;
  onSignedOut: () => void;
  client?: Api;
  stream?: StreamFactory;
}

/** The signed-in dashboard (spec §3–§6). */
export function Board({ onSignedOut, client = defaultApi, stream }: BoardProps) {
  const { state, dispatch, tooManyTabs, refetch } = useBoard({ onSignedOut, stream, client });
  const onBungieDown = useCallback(() => dispatch({ type: 'api_status', status: 'down' }), [dispatch]);
  const { refresh, cooldownUntil, busy } = useRefresh({ client, onBungieDown });
  const now = useNow(1000);
  const [managing, setManaging] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [removeError, setRemoveError] = useState<string | null>(null);

  const removal = useRemoval({
    remove: client.removeWatch,
    onRemoved: (id) => {
      dispatch({ type: 'remove', membershipId: id });
      void refetch();
    },
    onFailed: (p) => setRemoveError(`COULD NOT STOP WATCHING ${displayName(p).toUpperCase()}`),
  });

  const openPalette = useCallback(() => setPaletteOpen(true), []);
  const closePalette = useCallback(() => setPaletteOpen(false), []);
  usePaletteHotkeys(openPalette, !paletteOpen);

  useEffect(() => {
    if (!managing || paletteOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setManaging(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [managing, paletteOpen]);

  // When the undo bar goes away (undo or commit) its button takes focus with it; hand focus to a stable control.
  const rootRef = useRef<HTMLDivElement>(null);
  const hadPending = useRef(false);
  useEffect(() => {
    const had = hadPending.current;
    hadPending.current = removal.pending !== null;
    if (!had || removal.pending) return;
    const active = document.activeElement;
    if (active && active !== document.body) return;
    const root = rootRef.current;
    const target =
      root?.querySelector<HTMLElement>('button[aria-keyshortcuts="/"]') ??
      root?.querySelector<HTMLElement>('button[aria-label^="Stop watching"]') ??
      root?.querySelector<HTMLElement>('[data-focus-fallback]');
    target?.focus();
  }, [removal.pending]);

  const players = useMemo(
    () => [...state.players.values()].filter((p) => !removal.hidden.has(p.membershipId)),
    [state.players, removal.hidden],
  );
  const watchedIds = useMemo(() => new Set(state.players.keys()), [state.players]);
  const latest = latestSuccess(players);
  const cooldownLeft = Math.max(0, cooldownUntil - now);

  const signOut = async () => {
    try {
      await client.logout();
    } finally {
      onSignedOut();
    }
  };

  let main: ReactNode;
  if (!state.loaded) {
    main = <p className="t-status on-mist py-6">CONNECTING…</p>;
  } else if (players.length === 0 && !managing) {
    main = <EmptyBoard onOpen={openPalette} />;
  } else {
    main = (
      <BoardView
        players={players}
        now={now}
        managing={managing}
        onRemove={removal.start}
        onRecheck={() => void refresh()}
        recheckDisabled={cooldownLeft > 0 || busy}
        slot={managing ? null : <TrackSlot onOpen={openPalette} />}
      />
    );
  }

  return (
    <Backdrop>
      <div ref={rootRef} data-testid="board-root" className={removal.pending || removeError ? 'pb-40' : 'pb-24'}>
        <Header
          updatedAgo={latest === null ? null : now - latest}
          apiStatus={state.apiStatus}
          connected={state.connected || !state.loaded}
          cooldownLeft={cooldownLeft}
          refreshing={busy}
          onRefresh={() => void refresh()}
          managing={managing}
          onToggleManage={() => setManaging((m) => !m)}
          onSignOut={() => void signOut()}
          notices={
            <>
              <ApiBanner status={state.apiStatus} />
              <StreamNotice show={tooManyTabs} />
            </>
          }
        />
        <main className="px-4 pt-2 sm:px-10">{main}</main>
      </div>
      <Announcer players={players} />
      <PromptBar>
        <UndoBar pending={removal.pending} error={removeError} onUndo={removal.undo} onDismissError={() => setRemoveError(null)} />
        <span className="hidden items-center sm:inline-flex">
          <PromptGlyph>/</PromptGlyph>Track a player
        </span>
        <button type="button" onClick={openPalette} className="btn-primary px-3.5 py-2 text-[13px] sm:hidden">
          + Track
        </button>
      </PromptBar>
      <Palette open={paletteOpen} onClose={closePalette} client={client} watchedIds={watchedIds} onAdded={() => void refetch()} />
    </Backdrop>
  );
}
