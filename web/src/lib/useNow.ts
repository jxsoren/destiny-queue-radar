import { useEffect, useState } from 'react';

/** The current time in ms, re-rendering every intervalMs (timers tick every second, spec §3.5). */
export function useNow(intervalMs = 1000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(id);
  }, [intervalMs]);
  return now;
}
