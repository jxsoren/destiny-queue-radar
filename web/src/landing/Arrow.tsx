/** A drawn arrow, flat and square-ended, in the current text colour. */
export function Arrow({ size, stroke = 2 }: { size: number; stroke?: number }) {
  return (
    <svg aria-hidden="true" width={size} height={size} viewBox="0 0 24 24" className="arrow" fill="none">
      <path d="M3 12 H20 M13 5 L20 12 L13 19" stroke="currentColor" strokeWidth={stroke} strokeLinecap="square" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}
