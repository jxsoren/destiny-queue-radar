import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, test, vi } from 'vitest';
import type { Player, Status } from '../api/types';
import { NOW, iso, makePlayer } from '../test/fixtures';
import { PlayerCard, type PlayerCardProps } from './PlayerCard';

function renderCard(player: Player, props: Partial<PlayerCardProps> = {}) {
  render(
    <ul>
      <PlayerCard player={player} now={NOW} managing={false} {...props} />
    </ul>,
  );
  return screen.getByTestId(`card-${player.membershipId}`);
}
const withStatus = (status: Status, over: Partial<Player> = {}) => makePlayer({ status, ...over });

describe('PlayerCard', () => {
  test('name with the code split out, on the emblem banner from bungie.net', () => {
    const card = renderCard(makePlayer({ emblemBackgroundPath: '/common/destiny2_content/icons/vex_bg.jpg' }));
    expect(screen.getByText('Vex')).toBeInTheDocument();
    expect(screen.getByText('#1111')).toHaveClass('opacity-70');
    expect(screen.getByTestId('banner-art').style.backgroundImage).toContain('https://www.bungie.net/common/destiny2_content/icons/vex_bg.jpg');
    expect(card.querySelector('img')).toBeNull();
  });

  test('no banner, or a banner from anywhere else, falls back to the plain slate card', () => {
    expect(renderCard(makePlayer({ membershipId: 'a' })).querySelector<HTMLElement>('.banner-art')!.style.backgroundImage).toBe('');
    expect(renderCard(makePlayer({ membershipId: 'b', emblemBackgroundPath: 'https://evil.example/x.jpg' })).querySelector<HTMLElement>('.banner-art')!.style.backgroundImage).toBe('');
  });

  test('in match: green edge, playlist and map, live timer', () => {
    const card = renderCard(withStatus({ state: 'IN_MATCH', playlist: 'Trials of Osiris', activity: 'Burnout', startedAt: iso(252_000) }));
    expect(card).toHaveClass('is-live');
    expect(card).toHaveTextContent('TRIALS OF OSIRIS · BURNOUT');
    expect(card).toHaveTextContent('04:12');
  });

  test('offline group is greyscale; private shows the lock', () => {
    const card = renderCard(withStatus({ state: 'PRIVATE' }));
    expect(card).not.toHaveClass('is-live');
    expect(card).toHaveClass('is-muted');
    expect(card).toHaveTextContent('PRIVATE PROFILE');
    expect(screen.getByTestId('lock')).toBeInTheDocument();
  });

  test('stale marker after three minutes without a successful poll', () => {
    const card = renderCard(makePlayer({ lastSuccessAt: iso(240_000) }));
    expect(card).toHaveTextContent('STALE · 4M');
  });

  test('unknown state reads CHECKING…', () => {
    expect(renderCard(withStatus({ state: '' }, { lastPolledAt: null, lastSuccessAt: null }))).toHaveTextContent('CHECKING…');
  });

  test('manage mode: × on every card, ↻ only on NOT_FOUND', async () => {
    const user = userEvent.setup();
    const onRemove = vi.fn();
    const onRecheck = vi.fn();
    const player = withStatus({ state: 'NOT_FOUND' });
    renderCard(player, { managing: true, onRemove, onRecheck });
    await user.click(screen.getByRole('button', { name: 'Stop watching Vex#1111' }));
    expect(onRemove).toHaveBeenCalledWith(player);
    await user.click(screen.getByRole('button', { name: 'Re-check Vex#1111' }));
    expect(onRecheck).toHaveBeenCalledOnce();
  });

  test('↻ is disabled during the refresh cooldown and absent for other states', () => {
    renderCard(withStatus({ state: 'NOT_FOUND' }), { managing: true, recheckDisabled: true });
    expect(screen.getByRole('button', { name: 'Re-check Vex#1111' })).toBeDisabled();
  });

  test('no manage buttons outside manage mode', () => {
    renderCard(makePlayer());
    expect(screen.queryByRole('button')).toBeNull();
  });
});
