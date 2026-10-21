import { FLAVOR, ITEM_NAME, TYPE_LINE } from './copy';
import { LockGlyph } from './glyphs';
import { RadarMark } from './RadarMark';

/** The inspect screen's header: icon tile, name with the lock, type line, quoted flavour (spec §3.1). */
export function ItemHeader({ size = 'md' }: { size?: 'md' | 'lg' }) {
  const lg = size === 'lg';
  return (
    <div className="on-mist">
      <div className="flex items-start gap-4">
        <RadarMark size={lg ? 70 : 64} className="max-sm:h-[46px] max-sm:w-[46px]" />
        <div>
          <h1 className={`t-name flex items-center gap-3 ${lg ? 'text-[clamp(1.75rem,5vw,3.4rem)]' : 'text-[clamp(1.6rem,4.5vw,2.9rem)]'}`}>
            {ITEM_NAME}
            <span aria-hidden="true" className="grid h-5 w-5 shrink-0 place-items-center rounded-full border-2 border-white/85">
              <LockGlyph className="h-3 w-3" />
            </span>
          </h1>
          <p className="t-type mt-1 text-[clamp(0.75rem,1.6vw,1.1rem)]">{TYPE_LINE}</p>
        </div>
      </div>
      <p className="t-flavor mt-4 text-[15px]">“{FLAVOR}”</p>
    </div>
  );
}
