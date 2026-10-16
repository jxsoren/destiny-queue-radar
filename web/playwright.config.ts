import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 120_000,
  expect: { timeout: 20_000 },
  retries: 0, // the scenario changes server state; a retry would start from a dirty board
  workers: 1,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: 'https://localhost:9443',
    ignoreHTTPSErrors: true, // start.sh makes a throwaway self-signed certificate
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: 'bash e2e/start.sh',
    url: 'https://localhost:9443/api/me', // ready once it answers (401 counts)
    ignoreHTTPSErrors: true,
    timeout: 240_000,
    reuseExistingServer: false,
    stdout: 'pipe',
  },
});
