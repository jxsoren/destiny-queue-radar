import { useRef } from 'react';
import { Footer } from './Footer';
import { Hero } from './Hero';
import { Private } from './Private';
import { QueueWhileBusy } from './QueueWhileBusy';
import { SeeTheMatch } from './SeeTheMatch';
import { SignIn } from './SignIn';
import { useReveal } from './useReveal';
import { WatchThem } from './WatchThem';
import './landing.css';

/** The signed-out page: one name from your last lobby, followed until it turns red (spec §7). */
export function Landing() {
  const root = useRef<HTMLDivElement>(null);
  useReveal(root);
  return (
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
  );
}
