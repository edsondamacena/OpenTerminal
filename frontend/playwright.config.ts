import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: 'tests/e2e',
  testMatch: '**/*.spec.ts',
  workers: 1,
  retries: 0,
  timeout: 90_000,
  globalSetup: './tests/e2e/global-setup.ts',
  use: { trace: 'on-first-retry' },
});
