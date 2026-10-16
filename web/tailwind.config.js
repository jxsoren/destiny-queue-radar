/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './landing-preview.html', './src/**/*.{ts,tsx}'],
  theme: {
    // Tactical Telemetry: square corners only, colours only from the tokens in tokens.css.
    borderRadius: { none: '0' },
    colors: {
      transparent: 'transparent',
      current: 'currentColor',
      ground: 'var(--ground)',
      surface: 'var(--surface)',
      raised: 'var(--raised)',
      line: 'var(--line)',
      ink: 'var(--ink)',
      dim: 'var(--ink-dim)',
      stale: 'var(--ink-stale)',
      accent: 'var(--accent)',
      backdrop: 'var(--backdrop)',
    },
    fontFamily: {
      display: 'var(--font-display)',
      mono: 'var(--font-mono)',
      ui: 'var(--font-ui)',
    },
    extend: {
      boxShadow: { edge: 'inset 3px 0 0 0 var(--accent)' },
    },
  },
  plugins: [],
};
