import { defineConfig, devices } from '@playwright/test'

// Browser journeys against a running stack (see test/e2e/README.md).
// The product is French by default; the journeys assert the English source text.
const baseURL = process.env.E2E_APP ?? 'http://localhost:8080'
export default defineConfig({
  testDir: '.',
  testMatch: /.*\.spec\.mjs/,
  workers: 1,
  retries: 0,
  timeout: 90_000,
  reporter: [['list']],
  use: {
    baseURL,
    storageState: { cookies: [], origins: [{ origin: new URL(baseURL).origin, localStorage: [{ name: 'trezutil_lang', value: 'en' }] }] },
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 900 } } },
    { name: 'mobile', use: { ...devices['Pixel 7'] } },
  ],
})
