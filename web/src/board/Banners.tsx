import type { ApiStatus } from '../api/types';

/** Full-width red bar under the header while Bungie is down or degraded (spec §3.4). */
export function ApiBanner({ status }: { status: ApiStatus }) {
  if (status === 'ok') return null;
  return (
    <p role="alert" className="macro bg-accent px-4 py-3 text-xl text-ground md:text-2xl">
      {status === 'down' ? 'BUNGIE API UNAVAILABLE — STATUSES FROZEN' : 'BUNGIE API DEGRADED'}
    </p>
  );
}

/** Shown while the live stream is refused (most likely the per-user stream limit). */
export function StreamNotice({ show }: { show: boolean }) {
  // The live region stays mounted and only its text toggles, so a retry loop does not re-announce.
  return (
    <p role="status" className={show ? 'tele bg-raised px-4 py-2 text-ink' : 'sr-only'}>
      {show ? 'TOO MANY OPEN TABS · LIVE UPDATES PAUSED · RETRYING' : ''}
    </p>
  );
}
