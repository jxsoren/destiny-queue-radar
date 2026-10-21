/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './landing-preview.html', './src/**/*.{ts,tsx}'],
  theme: {
    // Inspect screen: square panels, circles only for perks and round buttons (spec §2.4).
    borderRadius: { none: '0', full: '9999px' },
    colors: {
      transparent: 'transparent',
      current: 'currentColor',
      white: '#ffffff',
      black: '#000000',
      'mist-hi': 'var(--mist-hi)',
      'mist-lo': 'var(--mist-lo)',
      gold: 'var(--gold-hi)',
      'perk-blue': 'var(--perk-blue)',
      live: 'var(--live)',
      'live-text': 'var(--live-text)',
      done: 'var(--done)',
      'tip-title': 'var(--tip-title)',
      'tip-text': 'var(--tip-text)',
      slate: 'var(--slate)',
      prompt: 'var(--prompt)',
      'prompt-ink': 'var(--prompt-ink)',
      dismiss: 'var(--dismiss)',
      stale: 'var(--stale)',
      warn: 'var(--warn)',
      scrim: 'var(--scrim)',
    },
    fontFamily: {
      ui: 'var(--font-ui)',
    },
    extend: {},
  },
  plugins: [],
};
