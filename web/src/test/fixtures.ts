import type { Match, Participant, Player } from '../api/types';

export const NOW = Date.parse('2026-10-15T18:00:00Z');

export function iso(msBeforeNow: number): string {
  return new Date(NOW - msBeforeNow).toISOString();
}

export function makePlayer(over: Partial<Player> = {}): Player {
  return {
    membershipType: 3,
    membershipId: '4611',
    bungieName: 'Vex#1111',
    emblemPath: '/common/destiny2_content/icons/vex.jpg',
    emblemBackgroundPath: '',
    status: { state: 'NOT_IN_PVP', place: 'ORBIT', activity: 'Orbit' },
    lastPolledAt: iso(10_000),
    lastSuccessAt: iso(10_000),
    source: 'search',
    addedAt: iso(3_600_000),
    ...over,
  };
}

export function makeParticipant(over: Partial<Participant> = {}): Participant {
  return {
    membershipType: 3,
    membershipId: '4611',
    bungieName: 'Vex#1111',
    emblemPath: '/common/destiny2_content/icons/vex.jpg',
    team: 16,
    watched: false,
    isSelf: false,
    ...over,
  };
}

export function makeMatch(over: Partial<Match> = {}): Match {
  return {
    instanceId: '9001',
    period: iso(41 * 60_000),
    activity: 'Javelin-4',
    playlist: 'Competitive',
    players: [makeParticipant()],
    ...over,
  };
}
