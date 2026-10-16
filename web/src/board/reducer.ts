import type { ApiStatus, Player, PlayerUpdate, Snapshot } from '../api/types';

export interface BoardState {
  players: ReadonlyMap<string, Player>;
  apiStatus: ApiStatus;
  /** The live stream is open and has delivered a snapshot. */
  connected: boolean;
  /** Players have been loaded at least once (snapshot or watchlist). */
  loaded: boolean;
}

export type BoardAction =
  | { type: 'snapshot'; snapshot: Snapshot }
  | { type: 'player_update'; update: PlayerUpdate }
  | { type: 'api_status'; status: ApiStatus }
  | { type: 'watchlist'; players: Player[] }
  | { type: 'remove'; membershipId: string }
  | { type: 'disconnected' };

export const initialBoard: BoardState = { players: new Map(), apiStatus: 'ok', connected: false, loaded: false };

const polledMs = (p: Player) => (p.lastPolledAt ? Date.parse(p.lastPolledAt) : -Infinity);

export function boardReducer(state: BoardState, action: BoardAction): BoardState {
  switch (action.type) {
    case 'snapshot':
      return {
        players: new Map(action.snapshot.players.map((p) => [p.membershipId, p])),
        apiStatus: action.snapshot.apiStatus,
        connected: true,
        loaded: true,
      };
    case 'player_update': {
      const { membershipId, status, polledAt } = action.update;
      const current = state.players.get(membershipId);
      if (!current) return state; // identity arrives with the watchlist refetch
      const players = new Map(state.players);
      players.set(membershipId, { ...current, status, lastPolledAt: polledAt, lastSuccessAt: polledAt });
      return { ...state, players };
    }
    case 'api_status':
      return { ...state, apiStatus: action.status };
    case 'watchlist': {
      // The stream only carries status changes, so names and new players come from here. Keep
      // a streamed status that is newer than the fetched row.
      const players = new Map<string, Player>();
      for (const fetched of action.players) {
        const known = state.players.get(fetched.membershipId);
        players.set(
          fetched.membershipId,
          known && polledMs(known) > polledMs(fetched)
            ? { ...fetched, status: known.status, lastPolledAt: known.lastPolledAt, lastSuccessAt: known.lastSuccessAt }
            : fetched,
        );
      }
      return { ...state, players, loaded: true };
    }
    case 'remove': {
      if (!state.players.has(action.membershipId)) return state;
      const players = new Map(state.players);
      players.delete(action.membershipId);
      return { ...state, players };
    }
    case 'disconnected':
      return { ...state, connected: false };
  }
}
