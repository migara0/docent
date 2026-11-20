import { defineConfig } from '@playwright/test';

const PORT = 18900;

// The tests run against the real Go server (built binary) on a temporary copy of testdata/fixtures/e2e.
export default defineConfig({
  testDir: 'e2e',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 30_000,
  reporter: [['list']],
  webServer: {
    command: 'node e2e/server.mjs',
    url: `http://127.0.0.1:${PORT}/_api/health`,
    reuseExistingServer: false,
    timeout: 120_000,
  },
  use: {
    baseURL: `http://127.0.0.1:${PORT}`,
    launchOptions: { executablePath: process.env.CHROME ?? '/usr/bin/google-chrome', args: ['--no-sandbox'] },
    viewport: { width: 1440, height: 900 },
    colorScheme: 'light',
    permissions: ['clipboard-read', 'clipboard-write'],
  },
});
