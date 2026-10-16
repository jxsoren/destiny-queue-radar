import type { Player } from '../api/types';
import { formatAgo } from '../lib/format';
import { columnOf } from './columns';
import { Emblem } from './Emblem';
import { LockIcon } from './LockIcon';
import { displayName, splitName } from './names';
import { staleFor } from './stale';
import { statusLine } from './statusLine';

export interface PlayerCardProps {
  player: Player;
  now: number;
  managing: boolean;
  onRemove?: (p: Player) => void;
  onRecheck?: () => void;
  recheckDisabled?: boolean;
}

export function PlayerCard({ player, now, managing, onRemove, onRecheck, recheckDisabled = false }: PlayerCardProps) {
  const column = columnOf(player.status.state);
  const muted = column === 'offline';
  const line = statusLine(player, now);
  const stale = staleFor(player, now);
  const label = displayName(player);
  const { name, code } = splitName(label);
  const ink = muted ? 'text-dim' : 'text-ink';

  return (
    <li
      data-testid={`card-${player.membershipId}`}
      className={`card-enter flex items-center gap-3 bg-surface px-3 py-3 ${column === 'match' ? 'shadow-edge' : ''}`}
    >
      <Emblem key={player.emblemPath} path={player.emblemPath} muted={muted} />
      <div className="min-w-0 flex-1">
        <p className={`truncate text-[15px] font-semibold leading-tight ${ink}`}>
          {name}
          <span className="font-normal text-dim">{code}</span>
        </p>
        <p className={`tele mt-1 flex flex-wrap items-center gap-x-2 ${ink}`}>
          {line.lock && <LockIcon />}
          <span>{line.text}</span>
          {line.timer && <span className="tabular-nums">{line.timer}</span>}
          {stale !== null && <span className="text-stale">STALE · {formatAgo(stale)}</span>}
        </p>
      </div>
      {managing && (
        <div className="flex shrink-0 gap-px bg-line">
          {player.status.state === 'NOT_FOUND' && (
            <button
              type="button"
              aria-label={`Re-check ${label}`}
              disabled={recheckDisabled}
              onClick={onRecheck}
              className="h-9 w-9 bg-raised text-base text-ink hover:bg-ink hover:text-ground disabled:text-dim disabled:hover:bg-raised"
            >
              ↻
            </button>
          )}
          <button
            type="button"
            aria-label={`Stop watching ${label}`}
            onClick={() => onRemove?.(player)}
            className="h-9 w-9 bg-raised text-lg text-ink hover:bg-accent hover:text-ground"
          >
            ×
          </button>
        </div>
      )}
    </li>
  );
}
