import type { ReactNode } from 'react';
import type { ApiStatus } from '../api/types';
import { formatAgo } from '../lib/format';
import { ItemHeader } from '../ui/ItemHeader';

export interface HeaderProps {
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
  /** Warning strips, drawn under the rarity band. */
  notices?: ReactNode;
}

/** The rarity band with the meta line, any warning strips, then the item header (spec §3.1). */
export function Header(p: HeaderProps) {
  const cooling = p.cooldownLeft > 0;
  const ok = p.apiStatus === 'ok';
  return (
    <header>
      <div className="band on-mist flex flex-wrap items-center justify-end gap-x-4 gap-y-2 px-4 py-2 text-[13px] sm:px-8">
        <p>{p.updatedAgo === null ? 'UPDATED —' : `UPDATED ${formatAgo(p.updatedAgo)} AGO`}</p>
        <p className={ok ? '' : 'is-alert bg-prompt px-2 py-0.5'}>
          <span aria-hidden="true" className={`mr-1.5 inline-block h-[7px] w-[7px] rounded-full ${ok ? 'bg-live' : 'bg-dismiss'}`} />
          BUNGIE {p.apiStatus.toUpperCase()}
        </p>
        {!p.connected && <p>RECONNECTING…</p>}
        <button type="button" className="btn-outline" disabled={cooling || p.refreshing} onClick={p.onRefresh}>
          {cooling ? `REFRESH ${Math.ceil(p.cooldownLeft / 1000)}S` : 'REFRESH'}
        </button>
        <button type="button" data-focus-fallback className="btn-outline" onClick={p.onToggleManage}>
          {p.managing ? 'DONE' : 'MANAGE'}
        </button>
        <button type="button" className="hover:underline" onClick={p.onSignOut}>
          SIGN OUT
        </button>
      </div>
      {p.notices}
      <div className="px-4 pt-6 sm:px-10">
        <ItemHeader />
      </div>
    </header>
  );
}
