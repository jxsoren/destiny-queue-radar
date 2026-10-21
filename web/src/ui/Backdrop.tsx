import type { ReactNode } from 'react';
import { RadarGlyph } from './glyphs';

/** Every screen's ground (spec §2.3): mist, concentric rings and the faded maker mark. */
export function Backdrop({ children }: { children: ReactNode }) {
  return (
    <div className="mist min-h-screen text-white">
      <div aria-hidden="true" className="rings">
        <i />
        <i />
        <i />
        <i />
      </div>
      <p aria-hidden="true" className="watermark">
        <RadarGlyph className="h-6 w-6 opacity-60" />
        Queue Radar
      </p>
      {children}
    </div>
  );
}
