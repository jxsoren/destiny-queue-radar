import type { Player } from '../api/types';
import { displayName } from '../board/names';

export interface UndoBarProps {
  pending: Player | null;
  error: string | null;
  onUndo: () => void;
  onDismissError: () => void;
}

/** The bottom bar: "STOPPED WATCHING <NAME> · UNDO", and removal errors. */
export function UndoBar({ pending, error, onUndo, onDismissError }: UndoBarProps) {
  if (!pending && !error) return null;
  return (
    <div className="fixed inset-x-0 bottom-0 z-30 grid gap-px bg-line">
      {pending && (
        <div role="status" className="tele flex items-center justify-between bg-raised px-4 py-3 text-ink">
          <span>STOPPED WATCHING {displayName(pending).toUpperCase()} ·</span>
          <button type="button" onClick={onUndo} className="px-3 py-1 text-accent underline hover:bg-accent hover:text-ground">
            UNDO
          </button>
        </div>
      )}
      {error && (
        <div role="alert" className="tele flex items-center justify-between bg-raised px-4 py-3 text-accent">
          <span>{error}</span>
          <button type="button" aria-label="Dismiss" onClick={onDismissError} className="px-3 py-1 text-ink">
            ×
          </button>
        </div>
      )}
    </div>
  );
}
