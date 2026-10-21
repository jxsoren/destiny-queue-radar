import { useState } from 'react';

const BUNGIE = 'https://www.bungie.net';

/** Bungie paths are relative to bungie.net; data:image/ URIs (the demo's drawn emblems) pass; nothing else is loaded. */
export function emblemUrl(path: string): string | null {
  if (path.startsWith('/') && !path.startsWith('//')) return /["'\\()\n\r]/.test(path) ? null : BUNGIE + path;
  if (path.startsWith('data:image/')) return /["\\\n\r]/.test(path) ? null : path;
  return null;
}

/** The player's emblem icon (spec §1: an emblem next to every name). Remount with key={path}. */
export function Emblem({ path, size = 36 }: { path: string; size?: number }) {
  const [failed, setFailed] = useState(false);
  const src = emblemUrl(path);
  const cls = `block shrink-0 bg-slate`;
  if (!src || failed) return <span aria-hidden="true" className={cls} style={{ width: size, height: size }} />;
  return <img src={src} alt="" width={size} height={size} loading="lazy" className={cls} onError={() => setFailed(true)} />;
}
