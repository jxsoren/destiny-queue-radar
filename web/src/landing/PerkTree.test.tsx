import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test } from 'vitest';
import { PerkTree } from './PerkTree';

test('Lobby Recall is selected and described by its tooltip, with the cost row', () => {
  render(<PerkTree />);
  const recall = screen.getByRole('button', { name: 'Lobby Recall' });
  expect(recall).toHaveAttribute('aria-pressed', 'true');
  expect(recall).toHaveAccessibleDescription(/Press \/ after a match/);
  expect(screen.getByRole('tooltip')).toHaveTextContent('Bungie sign-in');
});

test('hovering or focusing another perk shows its tooltip instead', async () => {
  const user = userEvent.setup();
  render(<PerkTree />);
  await user.hover(screen.getByRole('button', { name: 'Live Scan' }));
  expect(screen.getByRole('tooltip')).toHaveTextContent('Live Scan');
  act(() => screen.getByRole('button', { name: 'Queue Window' }).focus());
  expect(screen.getByRole('button', { name: 'Queue Window' })).toHaveAccessibleDescription(/may be queuing again/);
});

test('locked and decorative nodes are hidden from screen readers', () => {
  const { container } = render(<PerkTree />);
  expect(screen.getAllByRole('button')).toHaveLength(6);
  for (const node of container.querySelectorAll('.perk-locked, .perk-upgrade, .perk-progress')) {
    expect(node.closest('[aria-hidden="true"]')).not.toBeNull();
  }
});
