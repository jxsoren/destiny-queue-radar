import { useCallback, useRef, useState } from 'react';
import { ApiError, api as defaultApi, type Api } from '../api/client';

export const REFRESH_COOLDOWN_MS = 30_000;

export interface UseRefreshOptions {
  client?: Pick<Api, 'refresh'>;
  /** Called on 503 so the board shows the Bungie banner. */
  onBungieDown: () => void;
}

/** Manual refresh (main spec §4.4). cooldownUntil is an epoch-ms deadline; 0 means none. */
export function useRefresh({ client = defaultApi, onBungieDown }: UseRefreshOptions) {
  const [cooldownUntil, setCooldownUntil] = useState(0);
  const [busy, setBusy] = useState(false);
  const inFlight = useRef(false);

  const refresh = useCallback(async () => {
    if (inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    try {
      await client.refresh();
      setCooldownUntil(Date.now() + REFRESH_COOLDOWN_MS);
    } catch (err) {
      if (err instanceof ApiError && err.status === 429) {
        setCooldownUntil(Date.now() + (err.retryAfter ?? REFRESH_COOLDOWN_MS / 1000) * 1000);
      } else if (err instanceof ApiError && err.status === 503) {
        onBungieDown();
      }
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  }, [client, onBungieDown]);

  return { refresh, cooldownUntil, busy };
}
