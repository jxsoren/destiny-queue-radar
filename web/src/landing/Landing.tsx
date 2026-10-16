import { Footer } from './Footer';
import { Hero } from './Hero';
import { Private } from './Private';
import { QueueWhileBusy } from './QueueWhileBusy';
import { SeeTheMatch } from './SeeTheMatch';
import { SignIn } from './SignIn';
import { WatchThem } from './WatchThem';
import './landing.css';

/** The signed-out page: one name from your last lobby, followed until it turns red (spec §7). */
export function Landing() {
  return (
    <div className="landing">
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
