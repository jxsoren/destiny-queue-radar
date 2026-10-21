import type { ReactNode } from 'react';

/** The black prompt bar along the bottom (spec §2.3). Only real shortcuts belong in it. */
export function PromptBar({ children }: { children: ReactNode }) {
  return (
    <div data-testid="prompt-bar" className="prompt-bar">
      {children}
    </div>
  );
}

/** A button-glyph pill in front of a prompt's label. Decorative: the label carries the name. */
export function PromptGlyph({ children, red = false }: { children: ReactNode; red?: boolean }) {
  return (
    <span aria-hidden="true" className={`glyph ${red ? 'glyph-red' : ''}`}>
      {children}
    </span>
  );
}
