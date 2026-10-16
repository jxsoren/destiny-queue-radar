import { expect, test } from 'vitest';
import { makePlayer } from '../test/fixtures';
import { displayName, splitName } from './names';

test('splitName separates the code', () => {
  expect(splitName('Vex#1111')).toEqual({ name: 'Vex', code: '#1111' });
  expect(splitName('A#B#0042')).toEqual({ name: 'A#B', code: '#0042' });
  expect(splitName('NoCode')).toEqual({ name: 'NoCode', code: '' });
  expect(splitName('')).toEqual({ name: '', code: '' });
});

test('displayName falls back to the membership id', () => {
  expect(displayName(makePlayer({ bungieName: 'Vex#1111' }))).toBe('Vex#1111');
  expect(displayName(makePlayer({ bungieName: '', membershipId: '4611686018400000002' }))).toBe('PLAYER 000002');
});
