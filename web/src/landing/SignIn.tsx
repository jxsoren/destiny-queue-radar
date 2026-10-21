import { order } from './useReveal';

/** Sign in: one sentence and the one action (spec §5.2 item 5). */
export function SignIn() {
  return (
    <section aria-label="Sign in" className="landing-section cta on-mist">
      <p data-reveal style={order(0)} className="cta-sentence">
        Sign in with Bungie and press /.
      </p>
      <a href="/auth/login" data-reveal style={order(1)} className="btn-primary">
        Sign in with Bungie
      </a>
    </section>
  );
}
