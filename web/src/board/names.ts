/** Splits "Name#1234" so the code can be dimmed. */
export function splitName(bungieName: string): { name: string; code: string } {
  const i = bungieName.lastIndexOf('#');
  if (i <= 0) return { name: bungieName, code: '' };
  return { name: bungieName.slice(0, i), code: bungieName.slice(i) };
}

/** The name to show; players added while private have no Bungie Name yet. */
export function displayName(p: { bungieName: string; membershipId: string }): string {
  return p.bungieName || `PLAYER ${p.membershipId.slice(-6)}`;
}
