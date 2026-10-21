import { useState, type ComponentType } from 'react';
import { BoltGlyph, ClockGlyph, EyeGlyph, ListGlyph, LockGlyph, RadarGlyph, RecallGlyph, SearchGlyph } from '../ui/glyphs';
import { Tooltip, type TooltipCost } from '../ui/Tooltip';

interface Perk {
  id: string;
  name: string;
  kind: 'intrinsic' | 'alt' | 'active';
  Glyph: ComponentType<{ className?: string }>;
  body: string;
  cost?: TooltipCost;
}

// Every claim here matches current behaviour (spec §5.1); reword rather than ship anything untrue.
const PERKS: readonly Perk[] = [
  { id: 'radar-tuning', name: 'Radar Tuning', kind: 'intrinsic', Glyph: RadarGlyph, body: 'Checks every player you follow, about once a minute while your board is open.' },
  { id: 'name-search', name: 'Name Search', kind: 'alt', Glyph: SearchGlyph, body: 'Type a Bungie Name to find anyone not in your last lobby.' },
  {
    id: 'lobby-recall',
    name: 'Lobby Recall',
    kind: 'active',
    Glyph: RecallGlyph,
    body: 'Press / after a match. Everyone from your last lobby is already there.',
    cost: { label: 'Bungie sign-in', have: 1, need: 1 },
  },
  { id: 'live-scan', name: 'Live Scan', kind: 'active', Glyph: BoltGlyph, body: 'Your board updates on its own. No refreshing.' },
  { id: 'queue-window', name: 'Queue Window', kind: 'active', Glyph: ClockGlyph, body: 'Shows who just left a match and may be queuing again.' },
  { id: 'silent-running', name: 'Silent Running', kind: 'active', Glyph: LockGlyph, body: 'Only you can see your list. Nobody is told they’re followed.' },
];

const KIND_CLASS = { intrinsic: 'perk-intrinsic', alt: 'perk-alt', active: 'perk-active' } as const;
const tipId = (id: string) => `perk-tip-${id}`;

/** Locked nodes and gaps that fill out the tree, as on the real inspect screens. Decorative. */
function Filler() {
  const locked = (G: ComponentType<{ className?: string }>, key: string) => (
    <span key={key} aria-hidden="true" className="perk perk-locked max-sm:hidden">
      <G />
    </span>
  );
  const gap = (key: string) => <span key={key} aria-hidden="true" className="max-sm:hidden" />;
  return (
    <>
      {locked(EyeGlyph, 'r1-7')}
      {gap('r2-1')}
      <span aria-hidden="true" className="perk perk-alt max-sm:hidden">
        <ListGlyph />
      </span>
      {gap('r2-3')}
      {locked(EyeGlyph, 'r2-4')}
      <span aria-hidden="true" className="perk perk-upgrade max-sm:hidden" />
      {gap('r2-6')}
      {locked(BoltGlyph, 'r2-7')}
      {gap('r3-1')}
      {locked(ListGlyph, 'r3-2')}
      {gap('r3-3')}
      {locked(ClockGlyph, 'r3-4')}
      {gap('r3-5')}
      {gap('r3-6')}
      <span aria-hidden="true" className="perk perk-progress max-sm:hidden">
        <span>
          <BoltGlyph className="h-7 w-7" />
        </span>
      </span>
    </>
  );
}

/** The first screen's perk tree (spec §5.1): hover, focus or tap a perk to read its tooltip. */
export function PerkTree() {
  const [open, setOpen] = useState('lobby-recall');
  const current = PERKS.find((p) => p.id === open) ?? PERKS[2];
  return (
    <div className="on-mist">
      <div className="perk-tree">
        {PERKS.map((p) => (
          <button
            key={p.id}
            type="button"
            aria-label={p.name}
            aria-describedby={p.id === open ? tipId(p.id) : undefined}
            onMouseEnter={() => setOpen(p.id)}
            onFocus={() => setOpen(p.id)}
            onClick={() => setOpen(p.id)}
            className={`perk ${KIND_CLASS[p.kind]} ${p.id === open ? 'perk-selected' : ''} ${p.kind === 'active' ? '' : 'max-sm:hidden'}`}
          >
            <p.Glyph />
          </button>
        ))}
        <Filler />
      </div>
      <Tooltip id={tipId(current.id)} role="tooltip" title={current.name} cost={current.cost} className="perk-tip">
        {current.body}
      </Tooltip>
    </div>
  );
}
