/** Claim 6, "Sign in with Bungie": one sentence and the one action, on an almost empty screen. */
export function SignIn() {
  return (
    <section aria-label="Sign in" className="cta">
      <p className="cta-sentence">Sign in with Bungie, press / and pick the name you lost to.</p>
      <a href="/auth/login" className="button button-small">
        Sign in with Bungie
      </a>
    </section>
  );
}
