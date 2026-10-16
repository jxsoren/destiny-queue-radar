/** A count-up clock: "04:12", or "1:02:03" past an hour. Negative durations show 00:00. */
export function formatTimer(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  return h > 0 ? `${h}:${pad2(m)}:${pad2(s)}` : `${pad2(m)}:${pad2(s)}`;
}

/** The largest whole unit of a duration: "23S", "2M", "1H", "2D". */
export function formatAgo(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  if (s < 60) return `${s}S`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}M`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}H`;
  return `${Math.floor(h / 24)}D`;
}

/** Two-digit counts for telemetry readouts ("07 / 50"). */
export function pad2(n: number): string {
  return String(n).padStart(2, '0');
}

/** Milliseconds from an ISO time to now, or null when the time is missing or invalid. */
export function since(isoTime: string | null | undefined, now: number): number | null {
  if (!isoTime) return null;
  const t = Date.parse(isoTime);
  return Number.isNaN(t) ? null : now - t;
}
