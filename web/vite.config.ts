import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

// The Go server. Its certificate is self-signed in development, so only this proxy skips
// verification, and only for a local backend; the browser itself talks to the Vite server.
const backend = process.env.DQR_BACKEND ?? 'https://localhost:8443';
const insecure = /^https:\/\/(localhost|127\.0\.0\.1)(:|\/|$)/.test(backend);

export default defineConfig({
  plugins: [react()],
  build: {
    // Embedded by internal/webui (go:embed all:dist). Outside the Vite root, so empty it explicitly.
    outDir: '../internal/webui/dist/app',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': { target: backend, secure: !insecure },
      '/auth': { target: backend, secure: !insecure },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    restoreMocks: true,
  },
});
