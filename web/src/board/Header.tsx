import type { ApiStatus } from '../api/types';
import { formatAgo, pad2 } from '../lib/format';

export interface HeaderProps {
  watching: number;
  /** ms since the newest successful poll, or null. */
  updatedAgo: number | null;
  apiStatus: ApiStatus;
  connected: boolean;
  /** ms left on the refresh cooldown. */
  cooldownLeft: number;
  refreshing: boolean;
  onRefresh: () => void;
  managing: boolean;
  onToggleManage: () => void;
  onSignOut: () => void;
}

const cell = 'bg-ground px-3 py-2';
const button = `${cell} text-ink hover:bg-ink hover:text-ground disabled:text-dim disabled:hover:bg-ground`;

/** Wordmark left, telemetry right (spec §3.1). */
export function Header(p: HeaderProps) {
  const cooling = p.cooldownLeft > 0;
  return (
    <header className="grid gap-px bg-line lg:grid-cols-[1fr_auto]">
      <div className="bg-ground px-4 pb-3 pt-5">
        <h1 className="macro text-[clamp(2.75rem,8vw,6.5rem)]">Queue Radar</h1>
      </div>
      <div className="tele flex flex-wrap content-end gap-px bg-line">
        <p className={cell}>
          WATCHING <data value={p.watching}>{pad2(p.watching)}</data> / 50
        </p>
        <p className={cell}>{p.updatedAgo === null ? 'UPDATED —' : `UPDATED ${formatAgo(p.updatedAgo)} AGO`}</p>
        <p className={`${cell} ${p.apiStatus === 'ok' ? 'text-ink' : 'text-accent'}`}>BUNGIE {p.apiStatus.toUpperCase()}</p>
        {!p.connected && <p className={`${cell} text-dim`}>RECONNECTING…</p>}
        <button type="button" className={button} disabled={cooling || p.refreshing} onClick={p.onRefresh}>
          {cooling ? `REFRESH ${Math.ceil(p.cooldownLeft / 1000)}S` : 'REFRESH'}
        </button>
        <button type="button" className={button} onClick={p.onToggleManage}>
          {p.managing ? 'DONE' : 'MANAGE'}
        </button>
        <button type="button" className={button} onClick={p.onSignOut}>
          SIGN OUT
        </button>
      </div>
    </header>
  );
}
