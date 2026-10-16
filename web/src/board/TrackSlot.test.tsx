import { render, screen } from '@testing-library/react';
import { expect, test } from 'vitest';
import { TrackSlot } from './TrackSlot';

test('the key hint is decorative and exposed as a shortcut', () => {
  const { container } = render(<TrackSlot onOpen={() => {}} />);
  const button = screen.getByRole('button', { name: '+ TRACK A PLAYER' });
  expect(button).toHaveAttribute('aria-keyshortcuts', '/');
  expect(container.querySelector('kbd')).toHaveAttribute('aria-hidden', 'true');
});
