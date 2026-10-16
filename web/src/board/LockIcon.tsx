/** A padlock for private profiles (an SVG, since emoji are not allowed). */
export function LockIcon() {
  return (
    <svg data-testid="lock" aria-hidden="true" width="10" height="12" viewBox="0 0 10 12" className="shrink-0">
      <path d="M2 5V3.5a3 3 0 0 1 6 0V5" fill="none" stroke="currentColor" strokeWidth="1.5" />
      <rect x="0.75" y="5" width="8.5" height="6.25" fill="currentColor" />
    </svg>
  );
}
