import { useCallback, useEffect, useReducer, useRef, useState } from 'react';
import { ApiError, api as defaultApi, type Api } from '../api/client';
import type { ApiStatusUpdate, PlayerUpdate, Snapshot } from '../api/types';
import { boardReducer, initialBoard } from '../board/reducer';
import { STREAM_CLOSED, backoffDelay, browserStream, type StreamFactory } from './stream';

export interface UseBoardOptions {
  onSignedOut: () => void;
  stream?: StreamFactory;
  client?: Pick<Api, 'me' | 'watchlist'>;
}

/** Parses a frame's JSON; a malformed (or null) frame is skipped rather than thrown. */
function parse<T>(e: MessageEvent<string>): T | null {
  try {
    return (JSON.parse(e.data) as T | null) ?? null;
  } catch {
    return null;
  }
}

/**
 * The live board (spec §6). The browser's EventSource reconnects by itself after a network
 * error and every reconnect starts with a snapshot. EventSource cannot see HTTP status codes,
 * so a failure before any snapshot is diagnosed with /api/me: 401 means the session ended;
 * otherwise the stream is most likely at its per-user limit (429), so the board loads the
 * watchlist and reopens the stream with backoff.
 */
export function useBoard({ onSignedOut, stream = browserStream, client = defaultApi }: UseBoardOptions) {
  const [state, dispatch] = useReducer(boardReducer, initialBoard);
  const [tooManyTabs, setTooManyTabs] = useState(false);
  const signedOut = useRef(onSignedOut);
  useEffect(() => {
    signedOut.current = onSignedOut;
  });

  useEffect(() => {
    let disposed = false;
    let source: { close(): void } | null = null;
    let retry: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;

    const endSession = () => {
      disposed = true;
      source?.close();
      signedOut.current();
    };

    const diagnose = async () => {
      let signedIn = false;
      try {
        await client.me();
        signedIn = true;
      } catch (err) {
        if (disposed) return;
        if (err instanceof ApiError && err.status === 401) {
          endSession();
          return;
        }
      }
      if (disposed) return;
      if (signedIn) {
        setTooManyTabs(true);
        client.watchlist().then(
          (players) => !disposed && dispatch({ type: 'watchlist', players }),
          () => {},
        );
      }
      retry = setTimeout(open, backoffDelay(attempt++));
    };

    function open() {
      if (disposed) return;
      let gotSnapshot = false;
      const es = stream('/api/stream');
      source = es;
      es.addEventListener('snapshot', (e) => {
        const snapshot = parse<Snapshot>(e);
        if (!snapshot) return;
        gotSnapshot = true;
        attempt = 0;
        setTooManyTabs(false);
        dispatch({ type: 'snapshot', snapshot });
      });
      es.addEventListener('player_update', (e) => {
        const update = parse<PlayerUpdate>(e);
        if (update) dispatch({ type: 'player_update', update });
      });
      es.addEventListener('api_status', (e) => {
        const update = parse<ApiStatusUpdate>(e);
        if (update) dispatch({ type: 'api_status', status: update.bungie });
      });
      es.addEventListener('logout', endSession);
      es.onerror = () => {
        dispatch({ type: 'disconnected' });
        if (gotSnapshot && es.readyState !== STREAM_CLOSED) {
          gotSnapshot = false; // the browser is reconnecting; the next failure gets diagnosed
          return;
        }
        es.close();
        void diagnose();
      };
    }

    open();
    return () => {
      disposed = true;
      clearTimeout(retry);
      source?.close();
    };
  }, [stream, client]);

  const refetch = useCallback(async () => {
    try {
      dispatch({ type: 'watchlist', players: await client.watchlist() });
    } catch {
      // The stream or the next refetch catches up; a 401 is routed by the API client.
    }
  }, [client]);

  return { state, dispatch, tooManyTabs, refetch };
}
