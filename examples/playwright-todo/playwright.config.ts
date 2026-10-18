import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  reporter: 'list',
  outputDir: './test-results',
  use: {
    // BYOT collects these and shows them on the run.
    screenshot: 'only-on-failure',
  },
});
