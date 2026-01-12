import { defineConfig } from '@playwright/test'

// Default port - keep in sync with backend/cmd/cue/main.go DefaultPort
const DEFAULT_PORT = '31337'
const port = process.env.CUE_PORT || DEFAULT_PORT

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'list',
  use: {
    baseURL: `https://localhost:${port}`,
    trace: 'on-first-retry',
    ignoreHTTPSErrors: true,
  },
})
