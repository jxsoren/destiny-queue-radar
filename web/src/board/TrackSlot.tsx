/** The "+ Track a player" slot (spec §3.3): dashed 2px border, "/" as the key hint. */
export function TrackSlot({ onOpen, large = false }: { onOpen: () => void; large?: boolean }) {
  return (
    <div className="bg-ground p-3">
      <button
        type="button"
        onClick={onOpen}
        aria-keyshortcuts="/"
        className={`tele flex w-full items-center justify-between border-2 border-dashed border-dim px-4 text-dim hover:border-ink hover:text-ink ${large ? 'py-16 text-sm' : 'py-4'}`}
      >
        <span>+ TRACK A PLAYER</span>
        <kbd aria-hidden="true" className="border border-dim px-2 py-0.5 text-ink">/</kbd>
      </button>
    </div>
  );
}

/** An empty board: one large slot and the line from spec §3.3. */
export function EmptyBoard({ onOpen }: { onOpen: () => void }) {
  return (
    <div className="grid gap-px bg-line">
      <TrackSlot onOpen={onOpen} large />
      <p className="macro bg-ground px-4 py-8 text-[clamp(1.5rem,4vw,3rem)] text-ink">
        Press / to track the player who just beat you.
      </p>
    </div>
  );
}
