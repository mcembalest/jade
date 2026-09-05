import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
    { name: 'webkit', use: { browserName: 'webkit' } },
  ],
  workers: 2,
  timeout: 20_000,
  expect: { timeout: 5_000 },
  retries: 0,
  globalSetup: './tests/global-setup.ts',
  outputDir: '../../.tmp/test-results',
  reporter: [['list'], ['html', { outputFolder: '../../.tmp/test-report', open: 'never' }]],
  use: {
    headless: true,
    viewport: { width: 1240, height: 820 },
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
    video: 'retain-on-failure',
  },
});
