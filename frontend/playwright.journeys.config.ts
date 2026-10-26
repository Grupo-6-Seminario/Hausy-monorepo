import { defineConfig } from '@playwright/test';

import { ports } from './journeys/fixture';

// The journeys run the whole stack with nothing mocked: browser, the
// production frontend build, the Go API and Postgres. Only the model is a
// stand-in (e2e/journeys/fakemodel). Run them with `npm run test:journeys`,
// which seeds hausy_e2e_test first. Every server gets its own port, so a dev
// server on 3000 or an API on 8080 never answers for them.
const api = `http://127.0.0.1:${ports.api}`;

export default defineConfig({
  testDir: './journeys',
  testMatch: '**/*.journey.ts',
  outputDir: 'test-results/journeys/output',
  workers: 1,
  timeout: 60_000,
  reporter: [
    ['list'],
    ['html', { outputFolder: 'test-results/journeys/report', open: 'never' }],
    ['./journeys/timing-reporter.ts'],
  ],
  use: {
    baseURL: `http://127.0.0.1:${ports.frontend}`,
    channel: 'chrome',
    viewport: { width: 1440, height: 900 },
    trace: 'retain-on-failure',
  },
  webServer: [
    {
      command: `go run ../e2e/journeys/fakemodel -addr 127.0.0.1:${ports.model}`,
      url: `http://127.0.0.1:${ports.model}/publicacion/ready`,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    },
    {
      command: 'go run ../cmd/hausy',
      url: `${api}/api/eligibility/facts`,
      reuseExistingServer: !process.env.CI,
      timeout: 180_000,
      env: {
        HAUSY_API_ADDR: `127.0.0.1:${ports.api}`,
        DATABASE_URI: 'postgresql://hausy:hausy@localhost:5432/hausy_e2e_test',
        HAUSY_LLM: 'local',
        HAUSY_WRITER_LLM: 'local',
        LOCAL_LLM_URL: `http://127.0.0.1:${ports.model}`,
        // Blank, not inherited: the planner stays on the local model and
        // nothing reaches the AI Gateway.
        HAUSY_PLANNER: '',
        AI_GATEWAY_API_KEY: '',
      },
    },
    {
      command: `npm run build && npx wrangler dev --config dist/server/wrangler.json --port ${ports.frontend} --inspector-port ${ports.inspector} --var HAUSY_BACKEND_URL:${api}`,
      url: `http://127.0.0.1:${ports.frontend}`,
      reuseExistingServer: !process.env.CI,
      timeout: 300_000,
    },
  ],
});
