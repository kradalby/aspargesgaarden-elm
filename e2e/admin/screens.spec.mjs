// Screenshots of every screen at both widths, for a person to look at
// ($OUT, not compared).
import { test, expect } from "@playwright/test";
import { join } from "node:path";

const fixture = (name) => join(import.meta.dirname, "fixtures", name);
const file = (testInfo, name) =>
  join(process.env.OUT || "out", `${testInfo.project.name}-${name}.png`);
const shot = (page, testInfo, name) =>
  page.screenshot({ path: file(testInfo, name), fullPage: true });

test.beforeEach(async ({ request }) => {
  await request.post("/__reset");
});

test("screens", async ({ page }, testInfo) => {
  await page.goto("/admin/");
  await expect(
    page.getByRole("heading", { name: "Takkebakstkurs" }),
  ).toBeVisible();
  await shot(page, testInfo, "1-oversikt");

  await page.getByRole("link", { name: "Nytt arrangement" }).click();
  await page.getByLabel("Tittel").fill("Julemarked 2026");
  await page.getByLabel("Dato", { exact: true }).fill("2026-11-14");
  await page.getByLabel("Starter klokken (valgfritt)").fill("12:00");
  await page
    .getByLabel("Kort tekst")
    .fill("Velkommen til årets julemarked i Låven.");
  await page
    .locator("#hele-teksten")
    .fill("## Når og hvor\n\nLørdag og søndag i Låven.");
  const chooser = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "Velg bilde" }).click();
  await (await chooser).setFiles(fixture("konsert.jpg"));
  await expect(page.getByText("Klargjør bildet … 23 av 46")).toBeVisible();
  await page
    .locator("#bilde")
    .screenshot({ path: file(testInfo, "2a-klargjor") });
  await expect(page.locator('[data-photo-status="ready"]')).toBeVisible();
  await page.getByLabel("Beskriv bildet").fill("Boder i låven");
  await expect(page.locator("#forhandsvisning iframe")).toBeVisible();
  await shot(page, testInfo, "2-endre");

  await page.getByRole("button", { name: "Beskjær" }).click();
  await expect(
    page.getByRole("dialog", { name: "Beskjær bildet" }),
  ).toBeVisible();
  await page.waitForTimeout(500);
  await page.screenshot({ path: file(testInfo, "3-beskjaer") });
  await page.getByRole("button", { name: "Avbryt" }).click();

  await page.getByRole("button", { name: "Publiser" }).click();
  await expect(
    page.getByText("Publisert! Arrangementet er nå på nettsiden.").first(),
  ).toBeVisible();
  await shot(page, testInfo, "4-publisert");

  await page.getByRole("link", { name: "Galleri" }).click();
  await expect(page.getByText("Bilde 1 av 2")).toBeVisible();
  const files = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "+ Legg til bilder" }).click();
  await (
    await files
  ).setFiles([fixture("hage.jpg"), fixture("ikke-bilde.jpg")]);
  await expect(page.getByText("Bilde 3 av 3")).toBeVisible();
  await shot(page, testInfo, "5-galleri");
});
