import { fireEvent, render } from '@testing-library/react';
import { expect, test } from 'vitest';
import { Emblem, emblemUrl } from './Emblem';

test('bungie paths are prefixed; data images pass through', () => {
  expect(emblemUrl('/common/x.jpg')).toBe('https://www.bungie.net/common/x.jpg');
  expect(emblemUrl('data:image/svg+xml,%3Csvg%3E')).toBe('data:image/svg+xml,%3Csvg%3E');
});

test('anything else is refused', () => {
  for (const bad of ['', 'https://evil.example/x.png', 'http://www.bungie.net/x', 'javascript:alert(1)', 'data:text/html,<b>', '//evil.example/x.png', 'x.png'])
    expect(emblemUrl(bad), bad).toBeNull();
});

test('bungie paths that could break out of a css url() are refused', () => {
  for (const bad of ['/x"), url("https://evil.example/t', "/x'y", '/x\\y', '/x(1)', '/x\ny', '/x\ry'])
    expect(emblemUrl(bad), bad).toBeNull();
});

test('a broken emblem falls back to a plain square', () => {
  const { container } = render(<Emblem path="/common/x.jpg" />);
  fireEvent.error(container.querySelector('img')!);
  expect(container.querySelector('img')).toBeNull();
  expect(container.querySelector('span[aria-hidden="true"]')).not.toBeNull();
});

test('data URIs that could break out of a css url("…") are refused; parentheses are fine', () => {
  expect(emblemUrl('data:image/svg+xml,%3Csvg%20a="1"%3E')).toBeNull();
  expect(emblemUrl('data:image/png;base64,AA\\B')).toBeNull();
  expect(emblemUrl('data:image/png,a\nb')).toBeNull();
  const ok = 'data:image/svg+xml,%3Csvg%20transform%3D%22scale(2)%22%2F%3E';
  expect(emblemUrl(ok)).toBe(ok);
});
