/** The part of EventSource the board uses, so tests can inject a fake. */
export interface StreamLike {
  addEventListener(type: string, listener: (e: MessageEvent<string>) => void): void;
  close(): void;
  onerror: ((e: Event) => void) | null;
  readonly readyState: number;
}

export type StreamFactory = (url: string) => StreamLike;

export const STREAM_CLOSED = 2; // EventSource.CLOSED

export const browserStream: StreamFactory = (url) =>
  new EventSource(url, { withCredentials: true }) as unknown as StreamLike;

/** Reopen delays after the stream fails before a snapshot (spec §3.4): 10s, 20s, 40s, then 60s. */
export const BACKOFF_MS = [10_000, 20_000, 40_000, 60_000] as const;

export function backoffDelay(attempt: number): number {
  return BACKOFF_MS[Math.min(attempt, BACKOFF_MS.length - 1)];
}
