import { render, screen } from '@testing-library/react';
import { expect, test } from 'vitest';
import { Backdrop } from './Backdrop';
import { FLAVOR, TYPE_LINE } from './copy';
import { ItemHeader } from './ItemHeader';
import { PromptBar, PromptGlyph } from './PromptBar';
import { RadarMark } from './RadarMark';
import { Tooltip } from './Tooltip';

test('the item header: the name is the h1, then the type line and the quoted flavour', () => {
  render(<ItemHeader />);
  expect(screen.getByRole('heading', { level: 1, name: 'Queue Radar' })).toBeInTheDocument();
  expect(screen.getByText(TYPE_LINE)).toBeInTheDocument();
  expect(screen.getByText(`“${FLAVOR}”`)).toBeInTheDocument();
});

test('the radar mark is decorative, and two marks never share gradient ids', () => {
  const { container } = render(
    <>
      <RadarMark />
      <RadarMark />
    </>,
  );
  for (const mark of screen.getAllByTestId('radar-mark')) expect(mark).toHaveAttribute('aria-hidden', 'true');
  const ids = [...container.querySelectorAll('linearGradient')].map((g) => g.id);
  expect(new Set(ids).size).toBe(2);
});

test('prompt glyphs stay out of accessible names', () => {
  render(
    <PromptBar>
      <button type="button">
        <PromptGlyph>/</PromptGlyph>Track a player
      </button>
    </PromptBar>,
  );
  expect(screen.getByRole('button', { name: 'Track a player' })).toBeInTheDocument();
  expect(screen.getByTestId('prompt-bar')).toHaveClass('prompt-bar');
});

test('the tooltip has a title strip, a body and an optional cost row', () => {
  render(
    <Tooltip title="Lobby Recall" cost={{ label: 'Bungie sign-in', have: 1, need: 1 }}>
      Press / after a match.
    </Tooltip>,
  );
  expect(screen.getByText('Lobby Recall')).toHaveClass('tip-title');
  expect(screen.getByText('Press / after a match.')).toHaveClass('tip-text');
  expect(screen.getByText('Bungie sign-in')).toBeInTheDocument();
  expect(screen.getByText('/ 1')).toBeInTheDocument();
});

test('the backdrop draws decorative rings and the maker watermark around its children', () => {
  const { container } = render(
    <Backdrop>
      <p>inside</p>
    </Backdrop>,
  );
  expect(screen.getByText('inside')).toBeInTheDocument();
  expect(container.querySelectorAll('.rings > i')).toHaveLength(4);
  expect(container.querySelector('.watermark')).toHaveAttribute('aria-hidden', 'true');
});
