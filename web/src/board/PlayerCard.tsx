import type { CSSProperties } from 'react';
import type { Player } from '../api/types';
import { formatAgo } from '../lib/format';
import { columnOf } from './columns';
import { emblemUrl } from './Emblem';
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

/** One player on their emblem banner (spec §3.2). The banner art carries the emblem, so no icon is drawn. */
export function PlayerCard({ player, now, managing, onRemove, onRecheck, recheckDisabled = false }: PlayerCardProps) {
  const column = columnOf(player.status.state);
  const live = column === 'match';
  const muted = column === 'offline';
  const line = statusLine(player, now);
  const stale = staleFor(player, now);
  const label = displayName(player);
  const { name, code } = splitName(label);
  const banner = emblemUrl(player.emblemBackgroundPath);
  const style: CSSProperties | undefined = banner ? { backgroundImage: `url("${banner}")` } : undefined;

  return (
    <li
      data-testid={`card-${player.membershipId}`}
      className={`card-enter banner-card frame flex items-center gap-2 pl-[21%] pr-2 ${live ? 'is-live' : ''} ${muted ? 'is-muted' : ''}`}
    >
      <span aria-hidden="true" data-testid="banner-art" className="banner-art" style={style} />
      <div className="min-w-0 flex-1">
        <p className="text-shadow-strong truncate text-[15px] font-bold leading-tight">
          {name}
          <span className="font-normal opacity-70">{code}</span>
        </p>
        <p className={`t-status text-shadow-strong mt-0.5 flex flex-wrap items-center gap-x-2 ${live ? 'text-live-text' : ''}`}>
          {line.lock && <LockIcon />}
          <span>{line.text}</span>
          {line.timer && <span>{line.timer}</span>}
          {stale !== null && <span className="text-stale">STALE · {formatAgo(stale)}</span>}
        </p>
      </div>
      {managing && (
        <div className="flex shrink-0 gap-1.5">
          {player.status.state === 'NOT_FOUND' && (
            <button
              type="button"
              aria-label={`Re-check ${label}`}
              disabled={recheckDisabled}
              onClick={onRecheck}
              className="btn-round text-sm"
            >
              ↻
            </button>
          )}
          <button type="button" aria-label={`Stop watching ${label}`} onClick={() => onRemove?.(player)} className="btn-round text-base">
            ×
          </button>
        </div>
      )}
    </li>
  );
}
