import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test, vi } from 'vitest';
import { Header, type HeaderProps } from './Header';

function renderHeader(over: Partial<HeaderProps> = {}) {
  const props: HeaderProps = {
    updatedAgo: 23_000,
    apiStatus: 'ok',
    connected: true,
    cooldownLeft: 0,
    refreshing: false,
    onRefresh: vi.fn(),
    managing: false,
    onToggleManage: vi.fn(),
    onSignOut: vi.fn(),
    ...over,
  };
  render(<Header {...props} />);
  return props;
}

test('item header and meta readouts', () => {
  renderHeader();
  expect(screen.getByRole('heading', { level: 1, name: 'Queue Radar' })).toBeInTheDocument();
  expect(screen.getByText('Crucible Reconnaissance')).toBeInTheDocument();
  expect(screen.getByText('UPDATED 23S AGO')).toBeInTheDocument();
  expect(screen.getByText('BUNGIE OK')).not.toHaveClass('is-alert');
  expect(screen.queryByText('RECONNECTING…')).toBeNull();
});

test('no successful poll yet shows a dash', () => {
  renderHeader({ updatedAgo: null });
  expect(screen.getByText('UPDATED —')).toBeInTheDocument();
});

test('Bungie trouble is flagged', () => {
  renderHeader({ apiStatus: 'down' });
  expect(screen.getByText('BUNGIE DOWN')).toHaveClass('is-alert');
});

test('refresh shows the remaining cooldown and is disabled', async () => {
  const user = userEvent.setup();
  const props = renderHeader({ cooldownLeft: 12_400 });
  const button = screen.getByRole('button', { name: 'REFRESH 13S' });
  expect(button).toBeDisabled();
  await user.click(button);
  expect(props.onRefresh).not.toHaveBeenCalled();
});

test('refresh, manage and sign out buttons', async () => {
  const user = userEvent.setup();
  const props = renderHeader({ managing: true, connected: false });
  await user.click(screen.getByRole('button', { name: 'REFRESH' }));
  expect(props.onRefresh).toHaveBeenCalledOnce();
  const toggle = screen.getByRole('button', { name: 'DONE' });
  expect(toggle).not.toHaveAttribute('aria-pressed');
  await user.click(toggle);
  expect(props.onToggleManage).toHaveBeenCalledOnce();
  await user.click(screen.getByRole('button', { name: 'SIGN OUT' }));
  expect(props.onSignOut).toHaveBeenCalledOnce();
  expect(screen.getByText('RECONNECTING…')).toBeInTheDocument();
});

test('notices sit between the band and the item header', () => {
  renderHeader({ notices: <p>notice here</p> });
  const notice = screen.getByText('notice here');
  const heading = screen.getByRole('heading', { level: 1 });
  expect(notice.compareDocumentPosition(heading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
});
