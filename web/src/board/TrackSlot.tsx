/** The "+ Track a player" slot: a dashed white outline, "/" as the key hint (spec §3.1). */
export function TrackSlot({ onOpen, large = false }: { onOpen: () => void; large?: boolean }) {
  return (
    <button
      type="button"
      onClick={onOpen}
      aria-keyshortcuts="/"
      className={`t-head on-mist flex w-full items-center justify-center gap-2 border-2 border-dashed border-white/70 px-4 hover:border-white ${large ? 'py-16 text-sm' : 'py-3'}`}
    >
      <span>+ TRACK A PLAYER</span>
      <kbd aria-hidden="true" className="glyph mr-0 text-[10px]">
        /
      </kbd>
    </button>
  );
}

/** An empty board: one large slot and one line. */
export function EmptyBoard({ onOpen }: { onOpen: () => void }) {
  return (
    <div className="grid max-w-3xl gap-4">
      <TrackSlot onOpen={onOpen} large />
      <p className="t-name on-mist normal-case text-[clamp(1.5rem,4vw,2.6rem)]">Press / to track a player.</p>
    </div>
  );
}
