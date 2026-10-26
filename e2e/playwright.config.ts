import { defineConfig } from '@playwright/test'

// `make e2e` starts a separate stack on :3100 and runs these tests against it, so the demo accounts stay clean.
// Against another stack: BASE_URL=http://localhost:3000 npx playwright test (the tests will add their applications there).
export default defineConfig({
  testDir: '.',
  testMatch: '*.spec.ts',
  timeout: 90_000,
  expect: { timeout: 20_000 },
  fullyParallel: false,
  workers: 1,
  reporter: [['list']],
  use: {
    baseURL: process.env.BASE_URL ?? 'http://localhost:3000',
    channel: process.env.PW_CHANNEL ?? 'chrome', // uses the installed Chrome; set PW_CHANNEL= to use bundled Chromium
    locale: 'ru-RU',
    // The greeting after sign-in covers the screen for a few seconds; the tests that are not about it turn animation off.
    reducedMotion: 'reduce',
    viewport: { width: 1440, height: 900 },
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
})
