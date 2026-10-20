// Mirrors the JSON written by internal/web. Keep in sync with playerJSON, status.Status,
// snapshotJSON, engine.PlayerUpdate, engine.APIStatusUpdate, meJSON, matchJSON,
// participantJSON and searchResultJSON.

/** status.State; "" means not yet polled. */
export type State = '' | 'IN_MATCH' | 'QUEUED' | 'LIKELY_QUEUING' | 'NOT_IN_PVP' | 'OFFLINE' | 'PRIVATE' | 'NOT_FOUND';

/** status.Place; refines NOT_IN_PVP. */
export type Place = 'ORBIT' | 'SOCIAL' | 'PVE';

export interface Status {
  state: State;
  place?: Place;
  activity?: string;
  mode?: string;
  playlist?: string;
  startedAt?: string;
  lastPvpEndedAt?: string;
  lastPvpPlaylist?: string;
}

export type Source = 'search' | 'match';

export interface Player {
  membershipType: number;
  membershipId: string;
  /** "Name#1234", or "" when Bungie returned no name (private profile). */
  bungieName: string;
  /** Path relative to https://www.bungie.net, or "". */
  emblemPath: string;
  emblemBackgroundPath: string;
  status: Status;
  lastPolledAt: string | null;
  lastSuccessAt: string | null;
  source: Source;
  addedAt: string;
}

export type ApiStatus = 'ok' | 'degraded' | 'down';

export interface Snapshot {
  players: Player[];
  apiStatus: ApiStatus;
}

export interface PlayerUpdate {
  membershipId: string;
  status: Status;
  polledAt: string;
}

export interface ApiStatusUpdate {
  bungie: ApiStatus;
}

export interface Me {
  id: number;
  bungieName: string;
  membershipType: number;
  membershipId: string;
}

export interface Participant {
  membershipType: number;
  membershipId: string;
  bungieName: string;
  emblemPath: string;
  /** -1 when the activity has no teams. */
  team: number;
  watched: boolean;
  isSelf: boolean;
}

export interface Match {
  instanceId: string;
  /** When the match started. */
  period: string;
  /** The map. */
  activity: string;
  playlist: string;
  players: Participant[];
}

export interface SearchResult {
  membershipType: number;
  membershipId: string;
  bungieName: string;
  emblemPath: string;
}

export interface AddWatchRequest {
  membershipType: number;
  membershipId: string;
  source: Source;
}
