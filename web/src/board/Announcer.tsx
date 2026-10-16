import { useEffect, useRef, useState } from 'react';
import type { Player } from '../api/types';
import { columnChanges, columnMap } from './announce';
import type { ColumnId } from './columns';

/** One polite live region for status changes (spec §3.6); cards themselves stay quiet. */
export function Announcer({ players }: { players: readonly Player[] }) {
  const prev = useRef<Map<string, ColumnId> | null>(null);
  const [message, setMessage] = useState('');
  useEffect(() => {
    if (prev.current) {
      const changes = columnChanges(prev.current, players);
      if (changes.length > 0) setMessage(changes.join('. '));
    }
    prev.current = columnMap(players);
  }, [players]);
  return (
    <p role="status" aria-live="polite" className="sr-only">
      {message}
    </p>
  );
}
