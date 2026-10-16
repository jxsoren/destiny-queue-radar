import { LobbyFragment } from './fragments/Lobby';
import { Arrow } from './Arrow';
import { order } from './useReveal';

/** Claim 1, "You met a cheater": the lobby you just left, with the name you lost to picked. */
export function Hero() {
  return (
    <header className="hero">
      <div className="hero-bar">
        <p className="wordmark">Queue Radar</p>
        <a href="/auth/login" className="hero-signin">
          Sign in with Bungie
        </a>
      </div>
      <div className="hero-column">
        <h1 data-rise style={order(0)} className="hero-title">
          Queue while
          <br />
          the cheater plays
        </h1>
        <p data-rise style={order(1)} className="hero-sentence">
          Queue Radar privately watches the players you suspect of cheating and shows you when they’re in a PvP match.
        </p>
        <div data-rise style={order(2)} className="hero-actions">
          <a href="/auth/login" className="button">
            Sign in with Bungie
          </a>
          <a href="#board" className="text-link">
            See it running
            <Arrow size={14} />
          </a>
        </div>
        <div data-rise style={order(3)} className="hero-fragment">
          <div className="hero-drift">
            <LobbyFragment />
          </div>
        </div>
      </div>
    </header>
  );
}
