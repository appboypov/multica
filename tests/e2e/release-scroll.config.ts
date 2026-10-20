import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: '.', testMatch: 'release-scroll.spec.ts', timeout: 120000,
  workers: 1, retries: 0, reporter: 'line', outputDir: '../../test-results/release-scroll/playwright',
});
