import { defineConfig } from '@playwright/test'

// Runs against a live stack: `docker compose up -d` first (BASE_URL defaults to the nginx entry point).
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
