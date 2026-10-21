import type { ApiStatus } from '../api/types';

const strip = 'tip t-head flex items-center gap-3 border-t-0 px-4 py-2 sm:px-8';
const Diamond = () => <span aria-hidden="true" className="h-2 w-2 shrink-0 rotate-45 bg-warn" />;

/** The warning strip under the band while Bungie is down or degraded (spec §3.1). */
export function ApiBanner({ status }: { status: ApiStatus }) {
  if (status === 'ok') return null;
  return (
    <p role="alert" className={strip}>
      <Diamond />
      {status === 'down' ? 'BUNGIE API UNAVAILABLE — STATUSES FROZEN' : 'BUNGIE API DEGRADED'}
    </p>
  );
}

/** Shown while the live stream is refused (most likely the per-user stream limit). */
export function StreamNotice({ show }: { show: boolean }) {
  // The live region stays mounted and only its text toggles, so a retry loop does not re-announce.
  return (
    <p role="status" className={show ? strip : 'sr-only'}>
      {show && (
        <>
          <Diamond />
          TOO MANY OPEN TABS · LIVE UPDATES PAUSED · RETRYING
        </>
      )}
    </p>
  );
}
