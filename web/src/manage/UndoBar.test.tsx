import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test, vi } from 'vitest';
import { makePlayer } from '../test/fixtures';
import { UndoBar } from './UndoBar';

test('shows the pending removal with undo, and errors', async () => {
  const user = userEvent.setup();
  const onUndo = vi.fn();
  const onDismissError = vi.fn();
  const { rerender, container } = render(<UndoBar pending={makePlayer()} error={null} onUndo={onUndo} onDismissError={onDismissError} />);
  expect(screen.getByText('STOPPED WATCHING VEX#1111 ·')).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'UNDO' }));
  expect(onUndo).toHaveBeenCalledOnce();

  rerender(<UndoBar pending={null} error="COULD NOT STOP WATCHING VEX#1111" onUndo={onUndo} onDismissError={onDismissError} />);
  expect(screen.getByRole('alert')).toHaveTextContent('COULD NOT STOP WATCHING VEX#1111');
  await user.click(screen.getByRole('button', { name: 'Dismiss' }));
  expect(onDismissError).toHaveBeenCalledOnce();

  rerender(<UndoBar pending={null} error={null} onUndo={onUndo} onDismissError={onDismissError} />);
  expect(container).toBeEmptyDOMElement();
});
