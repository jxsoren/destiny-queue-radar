import { render, screen } from '@testing-library/react';
import { expect, test } from 'vitest';
import { Landing } from './Landing';

test('the primary action is Sign in with Bungie', () => {
  render(<Landing />);
  const links = screen.getAllByRole('link', { name: /sign in with bungie/i });
  expect(links.length).toBeGreaterThan(0);
  for (const link of links) expect(link).toHaveAttribute('href', '/auth/login');
});

test('says it is not affiliated with Bungie', () => {
  render(<Landing />);
  expect(screen.getByText(/not affiliated with Bungie/i)).toBeInTheDocument();
});
