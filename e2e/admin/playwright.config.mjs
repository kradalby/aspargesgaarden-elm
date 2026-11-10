// Browser checks of the admin app; run.sh starts the server and sets BASE.
// WebKit is the nearest to Safari on an iPad or iPhone that runs here.
import { defineConfig } from "@playwright/test";

const desktop = { viewport: { width: 1280, height: 900 } };
const phone = {
  viewport: { width: 390, height: 844 },
  isMobile: true,
  hasTouch: true,
  deviceScaleFactor: 2,
};

export default defineConfig({
  testDir: ".",
  testMatch: "*.spec.mjs",
  fullyParallel: false,
  workers: 1,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: [["list"]],
  outputDir: `${process.env.OUT || "out"}/results`,
  use: {
    baseURL: process.env.BASE,
    locale: "nb-NO",
    trace: "retain-on-failure",
  },
  projects: [
    { name: "desktop", use: { browserName: "chromium", ...desktop } },
    { name: "phone", use: { browserName: "chromium", ...phone } },
    { name: "desktop-webkit", use: { browserName: "webkit", ...desktop } },
    { name: "phone-webkit", use: { browserName: "webkit", ...phone } },
  ],
});
