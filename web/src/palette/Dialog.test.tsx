import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useRef, useState } from 'react';
import { expect, test } from 'vitest';
import { Dialog } from './Dialog';

function Harness() {
  const [open, setOpen] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        open
      </button>
      {open && (
        <Dialog onClose={() => setOpen(false)} labelledBy="dlg-title" initialFocus={input}>
          <h2 id="dlg-title">Track a player</h2>
          <input ref={input} aria-label="first" />
          <button type="button">last</button>
        </Dialog>
      )}
    </>
  );
}

test('modal, focused, trapped, closed by Esc, focus returned', async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const opener = screen.getByRole('button', { name: 'open' });
  await user.click(opener);

  const dialog = screen.getByRole('dialog', { name: 'Track a player' });
  expect(dialog).toHaveAttribute('aria-modal', 'true');
  expect(screen.getByLabelText('first')).toHaveFocus();

  await user.tab();
  expect(screen.getByRole('button', { name: 'last' })).toHaveFocus();
  await user.tab();
  expect(screen.getByLabelText('first')).toHaveFocus();
  await user.tab({ shift: true });
  expect(screen.getByRole('button', { name: 'last' })).toHaveFocus();

  await user.keyboard('{Escape}');
  expect(screen.queryByRole('dialog')).toBeNull();
  expect(opener).toHaveFocus();
});

test('clicking the dimmed backdrop closes it', async () => {
  const user = userEvent.setup();
  render(<Harness />);
  await user.click(screen.getByRole('button', { name: 'open' }));
  await user.click(screen.getByTestId('dialog-backdrop'));
  expect(screen.queryByRole('dialog')).toBeNull();
});
