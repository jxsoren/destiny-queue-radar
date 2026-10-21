import { PlayerCard } from '../board/PlayerCard';
import { StatsBlock } from '../board/StatsBlock';
import { PITCH } from '../ui/copy';
import { DotsGlyph } from '../ui/glyphs';
import { ItemHeader } from '../ui/ItemHeader';
import { PREVIEW_IN_MATCH, PREVIEW_QUEUING, SAMPLE_NOW } from './fragments/sample';
import { PerkTree } from './PerkTree';

// Sample counts that match the preview (spec §5.1).
const SAMPLE_COUNTS = { match: 2, queue: 1, idle: 3, offline: 2 };

/** The first screen: Queue Radar presented as an exotic item, with a live-looking board preview. */
export function Hero() {
  return (
    <header className="hero">
      <div className="band on-mist flex items-center justify-end px-4 sm:px-8">
        <a href="/auth/login" className="text-[13px] tracking-wide hover:underline">
          Sign in with Bungie
        </a>
      </div>
      <div className="hero-grid">
        <div>
          <ItemHeader size="lg" />
          <StatsBlock
            counts={SAMPLE_COUNTS}
            label="Sample counts"
            aside={
              <a href="#board" className="material">
                <span className="material-ring">
                  <DotsGlyph />
                </span>
                <span>
                  See it
                  <br />
                  running
                </span>
              </a>
            }
          />
          <PerkTree />
          <p className="hero-pitch on-mist">{PITCH}</p>
        </div>
        <div role="figure" aria-label="Sample players in a match and queuing" className="hero-preview">
          <p className="t-head on-mist mb-2">IN MATCH</p>
          <ul className="grid gap-2">
            {PREVIEW_IN_MATCH.map((p) => (
              <PlayerCard key={p.membershipId} player={p} now={SAMPLE_NOW} managing={false} />
            ))}
          </ul>
          <p className="t-head on-mist mb-2 mt-5">QUEUING</p>
          <ul className="grid gap-2">
            {PREVIEW_QUEUING.map((p) => (
              <PlayerCard key={p.membershipId} player={p} now={SAMPLE_NOW} managing={false} />
            ))}
          </ul>
        </div>
      </div>
    </header>
  );
}
