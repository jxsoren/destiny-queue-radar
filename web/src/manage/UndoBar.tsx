import { useEffect, useRef } from 'react';
import type { Player } from '../api/types';
import { displayName } from '../board/names';
import { PromptGlyph } from '../ui/PromptBar';

export interface UndoBarProps {
  pending: Player | null;
  error: string | null;
  onUndo: () => void;
  onDismissError: () => void;
}

/** The undo prompt and removal errors, drawn inside the black prompt bar (spec §3.3). */
export function UndoBar({ pending, error, onUndo, onDismissError }: UndoBarProps) {
  const undoRef = useRef<HTMLButtonElement>(null);
  // The × that started the removal disappears with its card; keep keyboard focus on the way back.
  useEffect(() => {
    if (pending) undoRef.current?.focus();
  }, [pending]);
  if (!pending && !error) return null;
  return (
    <div className="mr-auto flex flex-wrap items-center gap-x-5 gap-y-1">
      {pending && (
        <div role="status" className="flex items-center gap-2">
          <span id="undo-message">STOPPED WATCHING {displayName(pending).toUpperCase()} ·</span>
          <button ref={undoRef} aria-describedby="undo-message" type="button" onClick={onUndo} className="inline-flex items-center font-bold hover:underline">
            <PromptGlyph>↶</PromptGlyph>UNDO
          </button>
        </div>
      )}
      {error && (
        <div role="alert" className="flex items-center gap-2 text-[#ff8a85]">
          <span>{error}</span>
          <button type="button" aria-label="Dismiss" onClick={onDismissError} className="px-2 text-white">
            ×
          </button>
        </div>
      )}
    </div>
  );
}
