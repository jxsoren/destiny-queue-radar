import { describe, expect, test } from 'vitest';
import type { Status } from '../api/types';
import { NOW, iso, makePlayer } from '../test/fixtures';
import { statusLine } from './statusLine';

const line = (status: Status) => statusLine(makePlayer({ status }), NOW);

describe('statusLine', () => {
  test('unknown is checking', () => {
    expect(line({ state: '' })).toEqual({ text: 'CHECKING…', timer: null, lock: false });
  });

  test('in match: playlist, map and a live timer', () => {
    expect(line({ state: 'IN_MATCH', playlist: 'Trials of Osiris', activity: 'Burnout', startedAt: iso(252_000) })).toEqual({
      text: 'TRIALS OF OSIRIS · BURNOUT',
      timer: '04:12',
      lock: false,
    });
  });

  test('in match falls back to the mode and has no timer without a start', () => {
    expect(line({ state: 'IN_MATCH', mode: 'Control' })).toEqual({ text: 'CONTROL', timer: null, lock: false });
  });

  test('queuing', () => {
    expect(line({ state: 'LIKELY_QUEUING', place: 'ORBIT', lastPvpPlaylist: 'Competitive', lastPvpEndedAt: iso(120_000) }).text).toBe(
      'ORBIT · COMPETITIVE ENDED 2M AGO',
    );
    expect(line({ state: 'LIKELY_QUEUING', place: 'SOCIAL', activity: 'The Tower' }).text).toBe('THE TOWER · PVP ENDED RECENTLY');
  });

  test('not in pvp', () => {
    expect(line({ state: 'NOT_IN_PVP', place: 'ORBIT', lastPvpEndedAt: iso(3_600_000) }).text).toBe('ORBIT · LAST PVP 1H');
    expect(line({ state: 'NOT_IN_PVP', place: 'SOCIAL', activity: 'The Tower' }).text).toBe('THE TOWER');
    expect(line({ state: 'NOT_IN_PVP', place: 'PVE', activity: 'Vault of Glass' }).text).toBe('PVE · VAULT OF GLASS');
    expect(line({ state: 'NOT_IN_PVP' }).text).toBe('—');
  });

  test('offline, private, not found', () => {
    expect(line({ state: 'OFFLINE', lastPvpEndedAt: iso(2 * 86_400_000) }).text).toBe('OFFLINE · LAST PVP 2D');
    expect(line({ state: 'OFFLINE' }).text).toBe('OFFLINE');
    expect(line({ state: 'PRIVATE' })).toEqual({ text: 'PRIVATE PROFILE', timer: null, lock: true });
    expect(line({ state: 'NOT_FOUND' }).text).toBe('ACCOUNT NOT FOUND');
  });
});
