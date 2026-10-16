import type { Player } from '../api/types';
import { pad2 } from '../lib/format';
import type { ColumnDef } from './columns';
import { PlayerCard } from './PlayerCard';

export interface ColumnProps {
  def: ColumnDef;
  players: Player[];
  now: number;
  managing: boolean;
  hiddenOnPhone: boolean;
  onRemove?: (p: Player) => void;
  onRecheck?: () => void;
  recheckDisabled?: boolean;
}

export function Column({ def, players, now, managing, hiddenOnPhone, onRemove, onRecheck, recheckDisabled }: ColumnProps) {
  const headingId = `column-${def.id}`;
  return (
    <section aria-labelledby={headingId} className={`min-h-[8rem] bg-ground ${hiddenOnPhone ? 'hidden md:block' : ''}`}>
      <h2 id={headingId} className="tele flex justify-between px-3 py-2 text-dim">
        <span>{def.title}</span> <data value={players.length}>{pad2(players.length)}</data>
      </h2>
      <ul className="grid gap-px bg-line">
        {players.map((p) => (
          <PlayerCard
            key={p.membershipId}
            player={p}
            now={now}
            managing={managing}
            onRemove={onRemove}
            onRecheck={onRecheck}
            recheckDisabled={recheckDisabled}
          />
        ))}
      </ul>
    </section>
  );
}
