// Browser checks of the admin app. By default against e2e/fake-server.mjs;
// set ASP_ADMIN_URL (e.g. http://127.0.0.1:8080) to run against a real
// `asparges serve --dev`, which must then also answer the test hooks
// POST /__reset (seed data as in fake-server.mjs) and POST /__touch/<id>.
import { defineConfig } from "@playwright/test";

const port = 8791;
const external = process.env.ASP_ADMIN_URL;

export default defineConfig({
  testDir: ".",
  testMatch: "*.spec.mjs",
  fullyParallel: false,
  workers: 1,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: [["list"]],
  outputDir: "test-results",
  use: {
    baseURL: external || `http://127.0.0.1:${port}`,
    browserName: "chromium",
    locale: "nb-NO",
    trace: "retain-on-failure",
  },
  projects: [
    { name: "desktop", use: { viewport: { width: 1280, height: 900 } } },
    {
      name: "phone",
      use: {
        viewport: { width: 390, height: 844 },
        isMobile: true,
        hasTouch: true,
        deviceScaleFactor: 2,
      },
    },
  ],
  webServer: external
    ? undefined
    : {
        command: `node fake-server.mjs --port ${port}`,
        url: `http://127.0.0.1:${port}/admin/`,
        reuseExistingServer: false,
      },
});
