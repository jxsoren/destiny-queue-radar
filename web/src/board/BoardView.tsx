import { useMemo, useState, type ReactNode } from 'react';
import type { Player } from '../api/types';
import { Column } from './Column';
import { COLUMNS, groupPlayers, type ColumnId } from './columns';
import { CountsBar } from './CountsBar';

export interface BoardViewProps {
  players: readonly Player[];
  now: number;
  managing?: boolean;
  onRemove?: (p: Player) => void;
  onRecheck?: () => void;
  recheckDisabled?: boolean;
  /** The "+ Track a player" slot, rendered as the last grid cell. */
  slot?: ReactNode;
}

/** The four-column board, free of data fetching so the landing demo can reuse it. */
export function BoardView({ players, now, managing = false, onRemove, onRecheck, recheckDisabled, slot }: BoardViewProps) {
  const [phoneColumn, setPhoneColumn] = useState<ColumnId>('match');
  const groups = useMemo(() => groupPlayers(players), [players]);
  const counts: Record<ColumnId, number> = {
    match: groups.match.length,
    queue: groups.queue.length,
    idle: groups.idle.length,
    offline: groups.offline.length,
  };
  return (
    <div className="grid gap-px bg-line">
      <CountsBar counts={counts} phoneColumn={phoneColumn} onPhoneColumn={setPhoneColumn} />
      <div className="grid gap-px bg-line md:grid-cols-4">
        {COLUMNS.map((def) => (
          <Column
            key={def.id}
            def={def}
            players={groups[def.id]}
            now={now}
            managing={managing}
            hiddenOnPhone={def.id !== phoneColumn}
            onRemove={onRemove}
            onRecheck={onRecheck}
            recheckDisabled={recheckDisabled}
          />
        ))}
      </div>
      {slot}
    </div>
  );
}
