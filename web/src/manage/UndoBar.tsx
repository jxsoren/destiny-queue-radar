import { useEffect, useRef } from 'react';
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
  const undoRef = useRef<HTMLButtonElement>(null);
  // The × that started the removal disappears with its card; keep keyboard focus on the way back.
  useEffect(() => {
    if (pending) undoRef.current?.focus();
  }, [pending]);
  if (!pending && !error) return null;
  return (
    <div className="fixed inset-x-0 bottom-0 z-30 grid gap-px bg-line">
      {pending && (
        <div role="status" className="tele flex items-center justify-between bg-raised px-4 py-3 text-ink">
          <span id="undo-message">STOPPED WATCHING {displayName(pending).toUpperCase()} ·</span>
          <button ref={undoRef} aria-describedby="undo-message" type="button" onClick={onUndo} className="px-3 py-1 text-accent underline hover:bg-accent hover:text-ground">
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
