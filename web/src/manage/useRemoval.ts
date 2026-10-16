import { useCallback, useEffect, useRef, useState } from 'react';
import type { Player } from '../api/types';

export const UNDO_MS = 5_000;

export interface UseRemovalOptions {
  remove: (membershipId: string) => Promise<void>;
  /** The DELETE succeeded. */
  onRemoved: (membershipId: string) => void;
  /** The DELETE failed; the card is already visible again. */
  onFailed: (player: Player, err: unknown) => void;
  delayMs?: number;
}

/** Stop-watching with undo (spec §5). */
export function useRemoval({ remove, onRemoved, onFailed, delayMs = UNDO_MS }: UseRemovalOptions) {
  const [hidden, setHidden] = useState<ReadonlySet<string>>(new Set());
  const [pending, setPending] = useState<Player | null>(null);
  const pendingRef = useRef<Player | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout>>();
  const callbacks = useRef({ remove, onRemoved, onFailed });
  useEffect(() => {
    callbacks.current = { remove, onRemoved, onFailed };
  });

  const unhide = useCallback((id: string) => {
    setHidden((h) => {
      const next = new Set(h);
      next.delete(id);
      return next;
    });
  }, []);

  const take = () => {
    clearTimeout(timer.current);
    const p = pendingRef.current;
    pendingRef.current = null;
    setPending(null);
    return p;
  };

  const commit = useCallback(
    (p: Player) => {
      callbacks.current.remove(p.membershipId).then(
        () => {
          callbacks.current.onRemoved(p.membershipId);
          unhide(p.membershipId);
        },
        (err: unknown) => {
          unhide(p.membershipId);
          callbacks.current.onFailed(p, err);
        },
      );
    },
    [unhide],
  );

  const flush = useCallback(() => {
    const p = take();
    if (p) commit(p);
  }, [commit]);

  const start = useCallback(
    (p: Player) => {
      flush(); // a second removal sends the first one now
      pendingRef.current = p;
      setPending(p);
      setHidden((h) => new Set(h).add(p.membershipId));
      timer.current = setTimeout(flush, delayMs);
    },
    [flush, delayMs],
  );

  const undo = useCallback(() => {
    const p = take();
    if (p) unhide(p.membershipId);
  }, [unhide]);

  useEffect(
    () => () => {
      clearTimeout(timer.current);
      const p = pendingRef.current;
      pendingRef.current = null;
      if (p) callbacks.current.remove(p.membershipId).catch(() => {});
    },
    [],
  );

  return { hidden, pending, start, undo };
}
