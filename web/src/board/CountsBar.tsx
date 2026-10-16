import { pad2 } from '../lib/format';
import { COLUMNS, type ColumnId } from './columns';

export interface CountsBarProps {
  counts: Record<ColumnId, number>;
  phoneColumn: ColumnId;
  onPhoneColumn: (id: ColumnId) => void;
}

/** Big counts over mono labels (spec §3.2). On phones it becomes a sticky row of filter tabs. */
export function CountsBar({ counts, phoneColumn, onPhoneColumn }: CountsBarProps) {
  const hot = (id: ColumnId) => id === 'match' && counts.match > 0;
  return (
    <>
      <dl data-testid="counts" className="hidden grid-cols-4 gap-px bg-line md:grid">
        {COLUMNS.map((c) => (
          <div key={c.id} className="flex flex-col-reverse bg-ground px-4 py-4">
            <dt className="tele mt-2 text-dim">{c.label}</dt>
            <dd data-testid={`count-${c.id}`} className={`macro text-6xl ${hot(c.id) ? 'text-accent' : 'text-ink'}`}>
              {pad2(counts[c.id])}
            </dd>
          </div>
        ))}
      </dl>
      <div className="sticky top-0 z-10 grid grid-cols-4 gap-px bg-line md:hidden">
        {COLUMNS.map((c) => {
          const selected = phoneColumn === c.id;
          return (
            <button
              key={c.id}
              type="button"
              aria-pressed={selected}
              onClick={() => onPhoneColumn(c.id)}
              className={`border-b-2 px-2 py-2 text-left ${selected ? 'border-ink bg-raised' : 'border-transparent bg-ground'}`}
            >
              <span className={`macro block text-3xl ${hot(c.id) ? 'text-accent' : 'text-ink'}`}>{pad2(counts[c.id])}</span>
              <span className="tele block text-dim">{c.label}</span>
            </button>
          );
        })}
      </div>
    </>
  );
}
