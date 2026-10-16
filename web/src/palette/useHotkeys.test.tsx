import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test, vi } from 'vitest';
import { usePaletteHotkeys } from './useHotkeys';

function Harness({ onOpen, enabled = true }: { onOpen: () => void; enabled?: boolean }) {
  usePaletteHotkeys(onOpen, enabled);
  return <input aria-label="field" />;
}

test('"/" opens the palette outside text fields only', async () => {
  const user = userEvent.setup();
  const onOpen = vi.fn();
  render(<Harness onOpen={onOpen} />);
  await user.keyboard('/');
  expect(onOpen).toHaveBeenCalledOnce();
  await user.click(screen.getByLabelText('field'));
  await user.keyboard('/');
  expect(onOpen).toHaveBeenCalledOnce();
  expect(screen.getByLabelText('field')).toHaveValue('/');
});

test('Ctrl+K and ⌘K open it even in a text field', async () => {
  const user = userEvent.setup();
  const onOpen = vi.fn();
  render(<Harness onOpen={onOpen} />);
  await user.keyboard('{Control>}k{/Control}');
  await user.click(screen.getByLabelText('field'));
  await user.keyboard('{Meta>}k{/Meta}');
  expect(onOpen).toHaveBeenCalledTimes(2);
});

test('disabled hotkeys do nothing', async () => {
  const user = userEvent.setup();
  const onOpen = vi.fn();
  render(<Harness onOpen={onOpen} enabled={false} />);
  await user.keyboard('/');
  expect(onOpen).not.toHaveBeenCalled();
});
