import { useId } from 'react';

/** The logo: Destiny's HUD radar in an exotic-gold item tile (spec §1). Decorative. */
export function RadarMark({ size = 64, className = '' }: { size?: number; className?: string }) {
  const id = useId().replace(/:/g, '');
  return (
    <svg
      data-testid="radar-mark"
      aria-hidden="true"
      width={size}
      height={size}
      viewBox="0 0 72 72"
      className={`shrink-0 border-2 border-white ${className}`}
    >
      <defs>
        <linearGradient id={`${id}-gold`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#e2c440" />
          <stop offset="1" stopColor="#b59a2a" />
        </linearGradient>
        <radialGradient id={`${id}-scope`} cx=".5" cy=".5" r=".5">
          <stop offset="0" stopColor="#1f2a2c" />
          <stop offset="1" stopColor="#0d1314" />
        </radialGradient>
      </defs>
      <rect width="72" height="72" fill={`url(#${id}-gold)`} />
      <circle cx="36" cy="38" r="28" fill={`url(#${id}-scope)`} />
      <path d="M36 38 L16.2 18.2 A28 28 0 0 1 55.8 18.2 Z" fill="#fff" fillOpacity=".14" />
      <g fill="none" stroke="#e8eef0" strokeWidth="1.2">
        <path d="M16.2 18.2 A28 28 0 0 1 55.8 18.2" strokeOpacity=".9" strokeWidth="2" />
        <path d="M22.6 24.6 A19 19 0 0 1 49.4 24.6" strokeOpacity=".4" />
        <path d="M29.6 31.6 A9 9 0 0 1 42.4 31.6" strokeOpacity=".4" />
        <line x1="36" y1="38" x2="16.2" y2="18.2" strokeOpacity=".5" />
        <line x1="36" y1="38" x2="55.8" y2="18.2" strokeOpacity=".5" />
        <circle cx="36" cy="38" r="28" stroke="#fff" strokeWidth="2" />
      </g>
      <path d="M36 32.5 L40.5 42 L36 39.5 L31.5 42 Z" fill="#fff" />
      <path d="M43 18 L46.5 21 L43 24 L39.5 21 Z" fill="#ff5a3c" />
      <path d="M28 22 L30.6 24.2 L28 26.4 L25.4 24.2 Z" fill="#ff5a3c" opacity=".6" />
    </svg>
  );
}
