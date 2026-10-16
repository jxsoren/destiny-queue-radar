import { useEffect, useMemo, useReducer } from 'react';
import { BoardView } from '../../board/BoardView';
import { boardReducer, initialBoard } from '../../board/reducer';
import { useNow } from '../../lib/useNow';
import { demoPlayers, startDemo } from './script';

/** The landing page's live thing: the real board on a scripted stream (spec §7). */
export function DemoBoard() {
  const [state, dispatch] = useReducer(boardReducer, initialBoard, (s) =>
    boardReducer(s, { type: 'snapshot', snapshot: { players: demoPlayers(Date.now()), apiStatus: 'ok' } }),
  );
  useEffect(() => startDemo(dispatch), []);
  const now = useNow(1000);
  const players = useMemo(() => [...state.players.values()], [state.players]);
  return (
    <div data-ui aria-label="Sample board with made-up players" role="figure" className="bg-ground text-ink">
      <BoardView players={players} now={now} />
    </div>
  );
}
