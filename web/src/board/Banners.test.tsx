import { render, screen } from '@testing-library/react';
import { expect, test } from 'vitest';
import { ApiBanner, StreamNotice } from './Banners';

test('down and degraded banners', () => {
  const { rerender, container } = render(<ApiBanner status="down" />);
  expect(screen.getByRole('alert')).toHaveTextContent('BUNGIE API UNAVAILABLE — STATUSES FROZEN');
  rerender(<ApiBanner status="degraded" />);
  expect(screen.getByRole('alert')).toHaveTextContent('BUNGIE API DEGRADED');
  rerender(<ApiBanner status="ok" />);
  expect(container).toBeEmptyDOMElement();
});

test('too many tabs notice', () => {
  const { rerender } = render(<StreamNotice show />);
  expect(screen.getByRole('status')).toHaveTextContent('TOO MANY OPEN TABS');
  rerender(<StreamNotice show={false} />);
  expect(screen.getByRole('status')).toBeEmptyDOMElement();
});

test('the notice live region stays mounted so retries do not re-announce', () => {
  const { rerender } = render(<StreamNotice show={false} />);
  const region = screen.getByRole('status');
  expect(region).toBeEmptyDOMElement();
  rerender(<StreamNotice show />);
  expect(screen.getByRole('status')).toBe(region);
  expect(region).toHaveTextContent('TOO MANY OPEN TABS');
  rerender(<StreamNotice show={false} />);
  expect(screen.getByRole('status')).toBe(region);
  expect(region).toBeEmptyDOMElement();
});
