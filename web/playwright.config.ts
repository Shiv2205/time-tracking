import { defineConfig } from '@playwright/test'
import path from 'node:path'

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  workers: 1,
  timeout: 45000,
  use: {
    baseURL: 'http://127.0.0.1:8090',
    timezoneId: 'America/Toronto',
    viewport: { width: 1440, height: 1100 },
    screenshot: 'only-on-failure',
  },
  webServer: [
    {
      command: `"${path.resolve('../dist/timetracker.exe')}" -demo -port 8090 -data-dir "${path.resolve(`../.cache/e2e-${Date.now()}`)}"`,
      url: 'http://127.0.0.1:8090/api/bootstrap',
      timeout: 30000,
      reuseExistingServer: false,
    },
    {
      command: `"${path.resolve('../dist/timetracker.exe')}" -port 8091 -data-dir "${path.resolve(`../.cache/e2e-normal-${Date.now()}`)}"`,
      url: 'http://127.0.0.1:8091/api/bootstrap',
      timeout: 30000,
      reuseExistingServer: false,
    },
  ],
})
