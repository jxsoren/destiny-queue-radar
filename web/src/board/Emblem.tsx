import { useState } from 'react';

const BUNGIE = 'https://www.bungie.net';

/** Bungie paths are relative to bungie.net; anything else (demo data URIs) is used as is. */
export function emblemUrl(path: string): string | null {
  if (!path) return null;
  return path.startsWith('/') ? BUNGIE + path : path;
}

/** The player's emblem icon (spec §1: an emblem next to every name). Remount with key={path}. */
export function Emblem({ path, size = 36, muted = false }: { path: string; size?: number; muted?: boolean }) {
  const [failed, setFailed] = useState(false);
  const src = emblemUrl(path);
  const cls = `block shrink-0 bg-raised ${muted ? 'grayscale opacity-60' : ''}`;
  if (!src || failed) return <span aria-hidden="true" className={cls} style={{ width: size, height: size }} />;
  return <img src={src} alt="" width={size} height={size} loading="lazy" className={cls} onError={() => setFailed(true)} />;
}
