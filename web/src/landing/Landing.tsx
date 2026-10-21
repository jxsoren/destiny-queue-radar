import { useRef } from 'react';
import { Backdrop } from '../ui/Backdrop';
import { PromptBar, PromptGlyph } from '../ui/PromptBar';
import { Footer } from './Footer';
import { Hero } from './Hero';
import { Private } from './Private';
import { QueueWhileBusy } from './QueueWhileBusy';
import { SeeTheMatch } from './SeeTheMatch';
import { SignIn } from './SignIn';
import { useReveal } from './useReveal';
import { WatchThem } from './WatchThem';
import './landing.css';

/** The signed-out page, presented as a weapon inspect screen (spec §5). */
export function Landing() {
  const root = useRef<HTMLDivElement>(null);
  useReveal(root);
  return (
    <Backdrop>
      <div ref={root} className="landing">
        <main>
          <Hero />
          <WatchThem />
          <SeeTheMatch />
          <QueueWhileBusy />
          <Private />
          <SignIn />
        </main>
        <Footer />
      </div>
      <PromptBar>
        <a href="/auth/login" className="inline-flex items-center hover:underline">
          <PromptGlyph>↵</PromptGlyph>Sign in with Bungie
        </a>
        <a href="#board" className="inline-flex items-center hover:underline">
          <PromptGlyph red>↓</PromptGlyph>See it running
        </a>
      </PromptBar>
    </Backdrop>
  );
}
