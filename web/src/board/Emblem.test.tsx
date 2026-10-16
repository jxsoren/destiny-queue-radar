import { expect, test } from 'vitest';
import { emblemUrl } from './Emblem';

test('bungie paths are prefixed; data images pass through', () => {
  expect(emblemUrl('/common/x.jpg')).toBe('https://www.bungie.net/common/x.jpg');
  expect(emblemUrl('data:image/svg+xml,%3Csvg%3E')).toBe('data:image/svg+xml,%3Csvg%3E');
});

test('anything else is refused', () => {
  for (const bad of ['', 'https://evil.example/x.png', 'http://www.bungie.net/x', 'javascript:alert(1)', 'data:text/html,<b>', '//evil.example/x.png', 'x.png'])
    expect(emblemUrl(bad), bad).toBeNull();
});
