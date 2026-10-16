import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test, vi } from 'vitest';
import { Header, type HeaderProps } from './Header';

function renderHeader(over: Partial<HeaderProps> = {}) {
  const props: HeaderProps = {
    watching: 7,
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

test('wordmark and telemetry readouts', () => {
  renderHeader();
  expect(screen.getByRole('heading', { level: 1, name: 'Queue Radar' })).toBeInTheDocument();
  expect(screen.getByText(/WATCHING/)).toHaveTextContent('WATCHING 07 / 50');
  expect(screen.getByText('UPDATED 23S AGO')).toBeInTheDocument();
  expect(screen.getByText('BUNGIE OK')).not.toHaveClass('text-accent');
  expect(screen.queryByText('RECONNECTING…')).toBeNull();
});

test('no successful poll yet shows a dash', () => {
  renderHeader({ updatedAgo: null });
  expect(screen.getByText('UPDATED —')).toBeInTheDocument();
});

test('Bungie trouble is red', () => {
  renderHeader({ apiStatus: 'down' });
  expect(screen.getByText('BUNGIE DOWN')).toHaveClass('text-accent');
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
  expect(toggle).toHaveAttribute('aria-pressed', 'true');
  await user.click(toggle);
  expect(props.onToggleManage).toHaveBeenCalledOnce();
  await user.click(screen.getByRole('button', { name: 'SIGN OUT' }));
  expect(props.onSignOut).toHaveBeenCalledOnce();
  expect(screen.getByText('RECONNECTING…')).toBeInTheDocument();
});
