export function Landing() {
  return (
    <main className="min-h-screen bg-ground px-4 py-12 text-ink md:px-8">
      <h1 className="macro text-[clamp(3rem,10vw,9rem)]">Queue Radar</h1>
      <p className="mt-6 max-w-xl text-lg">
        Watch the players you suspect of cheating. See when they are in a match, and queue while they are busy.
      </p>
      <a href="/auth/login" className="mt-8 inline-block bg-accent px-6 py-4 font-semibold text-ground">
        Sign in with Bungie
      </a>
      <p className="mt-12 text-sm text-dim">Queue Radar is not affiliated with Bungie.</p>
    </main>
  );
}
