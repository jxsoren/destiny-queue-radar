import { useMemo, useState, type ReactNode } from 'react';
import type { Player } from '../api/types';
import { Column } from './Column';
import { COLUMNS, groupPlayers, type ColumnId } from './columns';
import { StatsBlock } from './StatsBlock';

export interface BoardViewProps {
  players: readonly Player[];
  now: number;
  managing?: boolean;
  onRemove?: (p: Player) => void;
  onRecheck?: () => void;
  recheckDisabled?: boolean;
  /** The "+ Track a player" slot, placed under the last column. */
  slot?: ReactNode;
}

/** Stats and the four columns, free of data fetching so the landing demo can reuse it. */
export function BoardView({ players, now, managing = false, onRemove, onRecheck, recheckDisabled, slot }: BoardViewProps) {
  const [filter, setFilter] = useState<ColumnId | null>(null);
  const groups = useMemo(() => groupPlayers(players), [players]);
  const counts: Record<ColumnId, number> = {
    match: groups.match.length,
    queue: groups.queue.length,
    idle: groups.idle.length,
    offline: groups.offline.length,
  };
  return (
    <div className="grid gap-4">
      <StatsBlock counts={counts} filter={filter} onFilter={setFilter} />
      <div className="grid gap-5 md:grid-cols-4 md:gap-4">
        {COLUMNS.map((def) => (
          <Column
            key={def.id}
            def={def}
            players={groups[def.id]}
            now={now}
            managing={managing}
            hiddenOnPhone={filter !== null && def.id !== filter}
            onRemove={onRemove}
            onRecheck={onRecheck}
            recheckDisabled={recheckDisabled}
          />
        ))}
      </div>
      {slot && (
        <div className="md:grid md:grid-cols-4 md:gap-4">
          <div className="md:col-start-4">{slot}</div>
        </div>
      )}
    </div>
  );
}
