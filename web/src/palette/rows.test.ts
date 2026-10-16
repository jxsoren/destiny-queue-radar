import { describe, expect, test } from 'vitest';
import { NOW, iso, makeMatch, makeParticipant } from '../test/fixtures';
import { buildGroups, flatten, isBungieName, lobbySummary } from './rows';

const last = makeMatch({
  instanceId: '9001',
  players: [
    makeParticipant({ membershipId: 'me', bungieName: 'Tester#0001', isSelf: true }),
    makeParticipant({ membershipId: 'v', bungieName: 'Vex#1111' }),
    makeParticipant({ membershipId: 'w', bungieName: 'Watched#0002', watched: true }),
  ],
});
const older = (n: number) =>
  makeMatch({
    instanceId: `90${10 + n}`,
    playlist: 'Control',
    activity: 'Endless Vale',
    period: iso(2 * 3_600_000),
    players: [makeParticipant({ membershipId: `k${n}`, bungieName: `Kestrel${n}#0412` }), makeParticipant({ membershipId: 'me', bungieName: 'Tester#0001', isSelf: true })],
  });

const base = { expanded: new Set<string>(), query: '', search: null, watched: new Set<string>(), now: NOW };

describe('isBungieName', () => {
  test.each([
    ['Vex#1111', true],
    [' Vex#1111 ', true],
    ['Ghost™ Rider#0880', true],
    ['Vex#111', false],
    ['Vex', false],
    ['#1111', false],
  ])('%s → %s', (q, want) => expect(isBungieName(q)).toBe(want));
});

test('lobbySummary', () => {
  expect(lobbySummary(makeMatch({ playlist: 'Competitive', activity: 'Javelin-4', period: iso(41 * 60_000), players: Array.from({ length: 6 }, (_, i) => makeParticipant({ membershipId: String(i) })) }), NOW)).toBe(
    'COMPETITIVE · JAVELIN-4 · 6 PLAYERS · 41M',
  );
});

describe('buildGroups', () => {
  test('last match lists every participant; self and watched are marked', () => {
    const g = buildGroups({ ...base, matches: [last], watched: new Set(['v-added-elsewhere']) });
    expect(g.last.map((r) => r.kind === 'player' && [r.membershipId, r.isSelf, r.watching, r.source])).toEqual([
      ['me', true, false, 'match'],
      ['v', false, false, 'match'],
      ['w', false, true, 'match'],
    ]);
    expect(g.older).toEqual([]);
    expect(g.search).toEqual([]);
  });

  test('the client watched set also marks players', () => {
    const g = buildGroups({ ...base, matches: [last], watched: new Set(['v']) });
    expect(g.last[1]).toMatchObject({ membershipId: 'v', watching: true });
  });

  test('older lobbies are matches 2-10, collapsed until expanded', () => {
    const matches = [last, ...Array.from({ length: 12 }, (_, i) => older(i))];
    const collapsed = buildGroups({ ...base, matches });
    expect(collapsed.older).toHaveLength(9);
    expect(collapsed.older.every((r) => r.kind === 'lobby' && !r.expanded)).toBe(true);

    const open = buildGroups({ ...base, matches, expanded: new Set(['9010']) });
    expect(open.older[0]).toMatchObject({ kind: 'lobby', instanceId: '9010', expanded: true });
    expect(open.older[1]).toMatchObject({ kind: 'player', membershipId: 'k0' });
    expect(open.older).toHaveLength(11);
  });

  test('a query filters by name and opens lobbies with matches', () => {
    const g = buildGroups({ ...base, matches: [last, older(0), older(1)], query: 'kestrel1' });
    expect(g.last).toEqual([]);
    expect(g.older.map((r) => (r.kind === 'lobby' ? `lobby:${r.instanceId}` : r.membershipId))).toEqual(['lobby:9011', 'k1']);
  });

  test('a search result becomes the search group', () => {
    const g = buildGroups({
      ...base,
      matches: [],
      query: 'Ghost#0880',
      search: { membershipType: 3, membershipId: '9', bungieName: 'Ghost#0880', emblemPath: '/g.jpg' },
    });
    expect(g.search).toEqual([
      expect.objectContaining({ kind: 'player', membershipId: '9', source: 'search', emblemPath: '/g.jpg', watching: false }),
    ]);
    expect(flatten(g)).toHaveLength(1);
  });

  test('flatten keeps group order', () => {
    const g = buildGroups({ ...base, matches: [last, older(0)] });
    expect(flatten(g).map((r) => r.key)).toEqual(['9001:me', '9001:v', '9001:w', '9010']);
  });
});
