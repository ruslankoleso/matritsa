import { defineConfig } from '@playwright/test'

// Records the demo film (make demo-video). Unlike the end-to-end tests, animations stay on and the pace is human.
export default defineConfig({
  testDir: '.',
  testMatch: 'record.spec.ts',
  timeout: 15 * 60_000,
  expect: { timeout: 20_000 },
  workers: 1,
  reporter: [['list']],
  use: {
    baseURL: process.env.BASE_URL ?? 'http://localhost:3100',
    channel: process.env.PW_CHANNEL ?? 'chrome',
    locale: 'ru-RU',
    timezoneId: 'Europe/Moscow',
    colorScheme: 'light',
  },
})
