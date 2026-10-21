// White-on-colour perk glyphs, drawn on a 30×30 grid. Always decorative.
type GlyphProps = { className?: string };

const svg = (className: string | undefined, children: React.ReactNode) => (
  <svg aria-hidden="true" viewBox="0 0 30 30" className={className}>
    {children}
  </svg>
);

export const RadarGlyph = ({ className }: GlyphProps) =>
  svg(
    className,
    <>
      <g fill="none" stroke="currentColor" strokeWidth="2.2">
        <path d="M5 21a12 12 0 0 1 20 0" />
        <path d="M9.5 21a7 7 0 0 1 11 0" />
      </g>
      <circle cx="15" cy="21" r="2" fill="currentColor" />
    </>,
  );

/** Lobby Recall (the user's pick, option D): a rewind arrow around a solid diamond. */
export const RecallGlyph = ({ className }: GlyphProps) =>
  svg(
    className,
    <>
      <path d="M24.6 15A9.6 9.6 0 1 1 20.8 7.4" fill="none" stroke="currentColor" strokeWidth="2.8" />
      <path d="M18 3.5l7.2 2.3-3.6 6.4z" fill="currentColor" />
      <path d="M15 10.2l4.8 4.8-4.8 4.8-4.8-4.8z" fill="currentColor" />
    </>,
  );

export const BoltGlyph = ({ className }: GlyphProps) => svg(className, <path d="M17 3L7 17h7l-2 10 10-14h-7z" fill="currentColor" />);

export const ClockGlyph = ({ className }: GlyphProps) =>
  svg(
    className,
    <g fill="none" stroke="currentColor" strokeWidth="2.2">
      <circle cx="15" cy="15" r="9" />
      <path d="M15 9v6l4 3" />
    </g>,
  );

export const LockGlyph = ({ className }: GlyphProps) =>
  svg(
    className,
    <g fill="none" stroke="currentColor" strokeWidth="2.2">
      <rect x="8" y="13" width="14" height="10" />
      <path d="M11 13v-3a4 4 0 0 1 8 0v3" />
    </g>,
  );

export const SearchGlyph = ({ className }: GlyphProps) =>
  svg(
    className,
    <g fill="none" stroke="currentColor" strokeWidth="2.4">
      <circle cx="13" cy="13" r="7" />
      <path d="M18 18l7 7" />
    </g>,
  );

export const ListGlyph = ({ className }: GlyphProps) =>
  svg(className, <path d="M7 9h16M7 15h16M7 21h10" fill="none" stroke="currentColor" strokeWidth="2.2" />);

export const EyeGlyph = ({ className }: GlyphProps) =>
  svg(
    className,
    <g fill="none" stroke="currentColor" strokeWidth="2.2">
      <path d="M3 15s4-7 12-7 12 7 12 7-4 7-12 7S3 15 3 15z" />
      <circle cx="15" cy="15" r="3" />
    </g>,
  );

export const DotsGlyph = ({ className }: GlyphProps) =>
  svg(
    className,
    <g fill="currentColor">
      <circle cx="15" cy="9" r="2.4" />
      <circle cx="9.5" cy="18" r="2.4" />
      <circle cx="20.5" cy="18" r="2.4" />
    </g>,
  );

/** A completed objective (spec §1): white check on solid green. */
export const CheckGlyph = ({ className }: GlyphProps) =>
  svg(className, <path d="M7 15.5l5.2 5.2L23.5 9.4" fill="none" stroke="currentColor" strokeWidth="3.4" strokeLinecap="square" />);
