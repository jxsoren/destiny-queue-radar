import { Fragment, type ReactNode } from 'react';
import { pad2 } from '../lib/format';
import { COLUMNS, type ColumnId } from './columns';

export const STAT_LABELS: Record<ColumnId, string> = {
  match: 'In match',
  queue: 'Queuing',
  idle: 'Not in PvP',
  offline: 'Offline',
};
export const WATCH_LIMIT = 50;

export interface StatsBlockProps {
  counts: Record<ColumnId, number>;
  /** Phones only: the one column shown, or null for all. Omit `onFilter` for a static block (landing). */
  filter?: ColumnId | null;
  onFilter?: (id: ColumnId | null) => void;
  /** The "Material details" slot on the right. */
  aside?: ReactNode;
  /** The server's watchlist size; defaults to the visible-card total. */
  watching?: number;
  /** The section's accessible name. */
  label?: string;
}

function Bar({ share, live, testId }: { share: number; live: boolean; testId?: string }) {
  return (
    <span aria-hidden="true" className="block h-3.5 bg-white/20">
      <span data-testid={testId} className={`block h-full ${live ? 'bg-live' : 'bg-white'}`} style={{ width: `${share}%` }} />
    </span>
  );
}

/** The inspect screen's stats (spec §3.1 item 5): big number, four bars, then the watchlist row. */
export function StatsBlock({ counts, filter = null, onFilter, aside, watching, label = 'Counts' }: StatsBlockProps) {
  const total = COLUMNS.reduce((n, c) => n + counts[c.id], 0);
  const share = (id: ColumnId) => (total === 0 ? 0 : Math.round((counts[id] / total) * 100));
  const live = (id: ColumnId) => id === 'match' && counts.match > 0;
  return (
    <section aria-label={label} className="on-mist">
      <hr className="divider" />
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:gap-8">
        <p className="t-big text-[clamp(2.6rem,6vw,3.75rem)]">
          <data value={total}>{pad2(total)}</data>
          <span className="mt-1 block text-sm font-normal tracking-wide">WATCHING</span>
        </p>
        <dl
          data-testid="counts"
          className={`${onFilter ? 'hidden md:grid' : 'grid'} t-label grid-cols-[6rem_minmax(6rem,10.5rem)_1.5rem] items-center gap-x-2.5 gap-y-1.5 text-[15px]`}
        >
          {COLUMNS.map((c) => (
            <Fragment key={c.id}>
              <dt className="text-right">{STAT_LABELS[c.id]}</dt>
              <dd aria-hidden="true">
                <Bar share={share(c.id)} live={live(c.id)} testId={`bar-${c.id}`} />
              </dd>
              <dd data-testid={`count-${c.id}`}>{counts[c.id]}</dd>
            </Fragment>
          ))}
        </dl>
        {onFilter && (
          <div role="group" aria-label="Show one column" className="grid gap-1 md:hidden">
            {COLUMNS.map((c) => {
              const on = filter === c.id;
              return (
                <button
                  key={c.id}
                  type="button"
                  aria-pressed={on}
                  aria-label={`${STAT_LABELS[c.id]} ${counts[c.id]}`}
                  onClick={() => onFilter(on ? null : c.id)}
                  className={`t-label grid grid-cols-[5.5rem_1fr_1.5rem] items-center gap-2 px-1.5 py-1 text-left text-[13px] ${on ? 'bg-white/15 outline outline-2 outline-white' : ''}`}
                >
                  <span className="text-right">{STAT_LABELS[c.id]}</span>
                  <Bar share={share(c.id)} live={live(c.id)} />
                  <span>{counts[c.id]}</span>
                </button>
              );
            })}
            <button
              type="button"
              aria-pressed={filter === null}
              onClick={() => onFilter(null)}
              className={`t-head justify-self-start px-1.5 py-1 ${filter === null ? 'underline' : ''}`}
            >
              All
            </button>
          </div>
        )}
        <p data-testid="watchlist" className="t-label text-[15px]">
          Watchlist {watching ?? total} / {WATCH_LIMIT}
        </p>
        {aside && <div className="sm:ml-auto">{aside}</div>}
      </div>
      <hr className="divider" />
    </section>
  );
}
