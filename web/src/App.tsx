import { useCallback, useEffect, useState } from 'react';
import { ApiError, api as defaultApi, setUnauthorizedHandler, type Api } from './api/client';
import type { Me } from './api/types';
import { Board } from './board/Board';
import { Landing } from './landing/Landing';
import type { StreamFactory } from './live/stream';

type Session = { kind: 'loading' } | { kind: 'signedOut' } | { kind: 'signedIn'; me: Me } | { kind: 'error' };

export interface AppProps {
  client?: Api;
  stream?: StreamFactory;
}

/** GET /api/me decides: 200 → Board, 401 → Landing, anything else → retry (spec §2). */
export function App({ client = defaultApi, stream }: AppProps) {
  const [session, setSession] = useState<Session>({ kind: 'loading' });

  const load = useCallback(() => {
    setSession({ kind: 'loading' });
    client.me().then(
      (me) => setSession({ kind: 'signedIn', me }),
      (err: unknown) =>
        setSession(err instanceof ApiError && err.status === 401 ? { kind: 'signedOut' } : { kind: 'error' }),
    );
  }, [client]);

  useEffect(() => {
    load();
  }, [load]);

  const signedOut = useCallback(() => setSession({ kind: 'signedOut' }), []);
  useEffect(() => {
    setUnauthorizedHandler(signedOut);
    return () => setUnauthorizedHandler(null);
  }, [signedOut]);

  switch (session.kind) {
    case 'loading':
      return <p className="t-status mist on-mist min-h-screen p-6">LOADING…</p>;
    case 'error':
      return (
        <main className="mist on-mist min-h-screen p-6 text-white">
          <p className="t-status">CAN&apos;T REACH QUEUE RADAR</p>
          <button type="button" onClick={load} className="btn-outline t-status mt-4">
            RETRY
          </button>
        </main>
      );
    case 'signedOut':
      return <Landing />;
    case 'signedIn':
      return <Board me={session.me} onSignedOut={signedOut} client={client} stream={stream} />;
  }
}
