// The admin app at desktop and phone size against `asparges serve --dev
// --e2e-fixtures` (run.sh), and the public pages the admin changes.
import { test as base, expect } from "@playwright/test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const fixture = (name) => join(import.meta.dirname, "fixtures", name);

// Every test starts from the seeded data (seed: the ids of its rows) and
// fails on any console error: a CSP violation, a script error, or a file
// that did not load.
const test = base.extend({
  seed: async ({ request }, use) => {
    const res = await request.post("/__reset");
    expect(res.ok()).toBe(true);
    await use(await res.json());
  },
  page: async ({ page, seed, browserName }, use) => {
    expect(seed.course).toBeGreaterThan(0);
    const problems = [];
    page.on("console", (m) => {
      if (m.type() !== "error") return;
      // An API refusal the test provokes (422, 409, an aborted request) is
      // logged by the browser too; the app's handling of it is what counts.
      if (
        /^Failed to load resource/.test(m.text()) &&
        (m.location().url.includes("/admin/api/") ||
          // A size the photo store has not built yet is missing for now;
          // the preview is drawn again once the photo is ready.
          m.location().url.includes("/img/"))
      )
        return;
      // Playwright reading the preview frame tries to run its own script in
      // it; the sandbox (no allow-scripts) refuses, as it should.
      if (m.text().startsWith("Blocked script execution in 'about:srcdoc'"))
        return;
      // WebKit reports its own <select> styling as refused by the CSP
      // (any <select> does, on any page with style-src 'self'); Chromium
      // still checks the app's own styles.
      if (
        browserName === "webkit" &&
        m.text().startsWith("Refused to apply a stylesheet because its hash")
      )
        return;
      problems.push(m.text());
    });
    page.on("pageerror", (e) => problems.push(String(e)));
    await use(page);
    expect(problems, "console errors").toEqual([]);
  },
});

// The public site as a visitor gets it.
async function publicText(request, path) {
  const res = await request.get(path);
  return { status: res.status(), text: await res.text() };
}

// The crop of a kind a page shows: the 8 hex digits in its image file names.
const cropOf = (html, kind) =>
  new RegExp(`/${kind}-([0-9a-f]{8})-`).exec(html)?.[1];

async function choose(page, button, files) {
  const chooser = page.waitForEvent("filechooser");
  await button.click();
  await (await chooser).setFiles(files);
}

async function noSideScroll(page) {
  // Against the device width: a phone widens its layout viewport to fit
  // overflowing content, so the document's own width proves nothing there.
  const wide = await page.evaluate((vw) => {
    return [...document.querySelectorAll("body *")]
      .filter((el) => el.getBoundingClientRect().right > vw + 0.5)
      .map(
        (el) =>
          `${el.tagName.toLowerCase()}#${el.id}.${String(el.className).slice(0, 60)}`,
      );
  }, page.viewportSize().width);
  expect(wide, "elements wider than the screen").toEqual([]);
}

// An event body for the API: a publishable draft with landskap as its
// main photo, changed by extra.
const draftInput = (seed, extra = {}) => ({
  updatedAt: "",
  slug: "",
  title: "Høstmarked",
  summary: "",
  body: "",
  start: "2026-11-21",
  startTime: "",
  end: "",
  endTime: "",
  extraDates: [],
  priceText: "",
  ticketUrl: "",
  showInMenu: false,
  menuLabel: "",
  cancelled: false,
  mainPhoto: { id: seed.landskap, alt: "Boder", photographerId: null },
  extraPhotos: [],
  ...extra,
});

const live = (page) => page.getByRole("status").filter({ hasNotText: /^$/ });

test("new event, from first word to published, copied and deleted", async ({
  page,
  seed,
  request,
}) => {
  await page.goto("/admin/");
  await expect(page.getByText("Logget inn som Utvikler")).toBeVisible();
  await noSideScroll(page);
  await page.getByRole("link", { name: "Nytt arrangement" }).click();
  await expect(page).toHaveURL(/\/admin\/arrangementer\/ny$/);

  await page.getByLabel("Tittel").fill("Julemarked 2026");
  await page.getByLabel("Dato", { exact: true }).fill("2026-11-14");
  await page.getByLabel("Starter klokken (valgfritt)").fill("12:00");
  await page.getByLabel("Siste dag (valgfritt)").fill("2026-11-15");
  await page.getByLabel("Slutter klokken (valgfritt)").fill("16:00");
  await page
    .getByLabel("Kort tekst")
    .fill("Velkommen til årets julemarked i Låven.");

  // Toolbar edits go through the textarea's own input event.
  const body = page.locator("#hele-teksten");
  await body.fill("Kom og handle julegaver");
  await body.selectText();
  await page.locator("md-bold").click();
  await expect(body).toHaveValue("**Kom og handle julegaver**");

  // Upload: the server checks the file, then prepares it while she goes on.
  await page.route(
    "**/admin/api/photos",
    async (route) => {
      await new Promise((r) => setTimeout(r, 800));
      await route.continue();
    },
    { times: 1 },
  );
  await choose(
    page,
    page.getByRole("button", { name: "Velg bilde" }),
    fixture("konsert.jpg"),
  );
  // Through a route the bytes may go unreported; either state is honest.
  await expect(
    page.getByText(/^(Laster opp …|Sjekker bildet …)/).first(),
  ).toBeVisible();
  // How long preparing takes, and whether she sees it at all, is the
  // server's business; it ends ready, and that is said aloud once.
  await expect(page.locator('[data-photo-status="ready"]')).toBeVisible({
    timeout: 60_000,
  });
  await expect(live(page).filter({ hasText: "klart" })).toHaveCount(1);
  // 1000 px wide: no share image, and she is told what is used instead.
  await expect(
    page.getByText("får ikke eget bilde for deling på Facebook"),
  ).toBeVisible();
  await page.getByLabel("Beskriv bildet").fill("Boder med julepynt i låven");

  // Crop with the buttons, so the keyboard path is the one tested.
  await page.getByRole("button", { name: "Beskjær" }).click();
  const crop = page.getByRole("dialog", { name: "Beskjær bildet" });
  await expect(crop).toBeVisible();
  await expect(crop.getByRole("button", { name: "Bruk" })).toBeFocused();
  await crop.getByRole("button", { name: /Øverst på siden/ }).click();
  await expect(
    crop.getByText("Toppen: Øverst på siden til arrangementet"),
  ).toBeVisible();
  await expect(crop.locator('[data-crop-unusable="share"]')).toContainText(
    "under 1200 punkter",
  );
  const cropSaved = page.waitForResponse(
    (r) => r.url().endsWith("/crops") && r.request().method() === "PUT",
  );
  await crop.getByRole("button", { name: "Mindre" }).click();
  await crop.getByRole("button", { name: "Flytt rammen ned" }).click();
  await crop.getByRole("button", { name: "Bruk" }).click();
  const saved = await cropSaved;
  expect(saved.status()).toBe(200);
  const sent = saved.request().postDataJSON();
  expect(Object.keys(sent).sort()).toEqual(["banner", "card", "share"]);
  expect(sent.banner.w).toBeLessThan(1000);
  await expect(crop).toBeHidden();
  await expect(page.getByRole("button", { name: "Beskjær" })).toBeFocused();

  // The preview is the server's page for the unsaved form, with its photo
  // and its stylesheet loaded inside the frame.
  const frame = page.frameLocator("#forhandsvisning iframe");
  await expect(
    frame.getByRole("heading", { name: "Julemarked 2026" }),
  ).toBeVisible();
  await expect
    .poll(() =>
      frame
        .locator("img")
        .first()
        .evaluate((i) => i.complete && i.naturalWidth),
    )
    .toBeGreaterThan(0);
  await expect
    .poll(() =>
      frame
        .locator("body")
        .evaluate(() =>
          [...document.styleSheets].reduce(
            (n, sh) => n + sh.cssRules.length,
            0,
          ),
        ),
    )
    .toBeGreaterThan(0);
  await page.getByLabel("Tittel").fill("Julemarked på gården");
  await expect(
    frame.getByRole("heading", { name: "Julemarked på gården" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "I listen" }).click();
  await expect(frame.locator('[data-preview="list"]')).toBeVisible();
  // The new event's card is first in the list and loads eagerly.
  const card = frame.locator('img[alt="Boder med julepynt i låven"]').first();
  await expect
    .poll(() => card.evaluate((i) => i.complete && i.naturalWidth))
    .toBeGreaterThan(0);
  await page.getByRole("button", { name: "PC" }).click();
  await expect(page.locator('asp-fit[width="1280"]')).toBeVisible();
  await noSideScroll(page);

  await page.getByRole("button", { name: "Lagre kladd" }).click();
  await expect(
    page.getByText("Kladden er lagret. Den vises ikke på nettsiden.").first(),
  ).toBeVisible();
  await expect(page).toHaveURL(/\/admin\/arrangementer\/\d+$/);
  await expect(page.getByText("Ikke lagret")).toBeHidden();
  await noSideScroll(page);

  await page.getByRole("button", { name: "Publiser" }).click();
  await expect(
    page.getByText("Publisert! Arrangementet er nå på nettsiden.").first(),
  ).toBeVisible();
  await expect(page.getByText("Publisert – vises på nettsiden")).toBeVisible();
  const editorUrl = page.url();
  const path = new URL(
    await page
      .getByRole("link", { name: /Se siden på nettsiden/ })
      .getAttribute("href"),
  ).pathname;
  expect(path).toBe("/arrangementer/2026/julemarked-pa-garden");

  // Published: on the list, the sitemap and its own page, with the crop.
  expect((await publicText(request, "/arrangementer")).text).toContain(
    "Julemarked på gården",
  );
  expect((await publicText(request, "/sitemap.xml")).text).toContain(path);
  const before = await publicText(request, path);
  expect(before.status).toBe(200);
  const banner = cropOf(before.text, "banner");
  expect(banner).toBeTruthy();

  // A new crop shows on the live page at once, under a new file name.
  await page.getByRole("button", { name: "Beskjær" }).click();
  await crop.getByRole("button", { name: /Øverst på siden/ }).click();
  await crop.getByRole("button", { name: "Flytt rammen opp" }).click();
  const recropped = page.waitForResponse((r) => r.url().endsWith("/crops"));
  await crop.getByRole("button", { name: "Bruk" }).click();
  expect((await recropped).status()).toBe(200);
  const after = cropOf((await publicText(request, path)).text, "banner");
  expect(after).toBeTruthy();
  expect(after).not.toBe(banner);

  await page
    .getByRole("link", { name: "Arrangementer", exact: true })
    .first()
    .click();
  const upcoming = page.getByRole("region", { name: "Kommende" });
  await expect(
    upcoming.getByRole("heading", { name: "Julemarked på gården" }),
  ).toBeVisible();
  await expect(
    upcoming.getByText(
      "14.–15. november 2026 kl. 12.00–16.00 på Aspargesgården",
    ),
  ).toBeVisible();

  await upcoming
    .getByRole("button", { name: "Lag kopi av Julemarked på gården" })
    .click();
  await expect(page).toHaveURL(/\/admin\/arrangementer\/\d+$/);
  await expect(page.getByLabel("Tittel")).toHaveValue("Julemarked på gården");
  await expect(page.getByLabel("Dato", { exact: true })).toHaveValue("");
  await expect(page.getByText("Kladd – vises ikke på nettsiden")).toBeVisible();

  await page
    .getByRole("link", { name: "Arrangementer", exact: true })
    .first()
    .click();
  const drafts = page.getByRole("region", { name: "Kladder" });
  await drafts
    .getByRole("button", { name: "Slett Julemarked på gården" })
    .click();
  const confirm = page.getByRole("dialog", { name: "Slette arrangementet?" });
  await expect(
    confirm.getByRole("button", { name: "Nei, behold det" }),
  ).toBeFocused();
  await confirm.getByRole("button", { name: "Ja, slett" }).click();
  await expect(drafts.getByText("Ingen kladder.")).toBeVisible();
  await expect(
    upcoming.getByRole("heading", { name: "Julemarked på gården" }),
  ).toBeVisible();

  // Back to a draft: off the public site, its address kept for later.
  await page.goto(editorUrl);
  await page.getByRole("button", { name: "Gjør om til kladd" }).click();
  await expect(page.getByText("Kladd – vises ikke på nettsiden")).toBeVisible();
  expect((await publicText(request, "/arrangementer")).text).not.toContain(
    "Julemarked på gården",
  );
  expect((await publicText(request, "/sitemap.xml")).text).not.toContain(path);
  expect((await publicText(request, path)).status).toBe(404);

  // Deleted: gone from the admin too.
  await page
    .getByRole("link", { name: "Arrangementer", exact: true })
    .first()
    .click();
  await drafts
    .getByRole("button", { name: "Slett Julemarked på gården" })
    .click();
  await confirm.getByRole("button", { name: "Ja, slett" }).click();
  await expect(drafts.getByText("Ingen kladder.")).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Julemarked på gården" }),
  ).toHaveCount(0);
});

test("warns before leaving with unsaved changes", async ({ page, seed }) => {
  await page.goto("/admin/");
  await page.getByRole("link", { name: "Endre Takkebakstkurs" }).click();
  await expect(page.getByLabel("Tittel")).toHaveValue("Takkebakstkurs");
  await page.getByLabel("Tittel").fill("Takkebakstkurs med Anita");

  // A link inside the app asks first, and staying keeps the text.
  await page.getByRole("link", { name: "Galleri" }).click();
  const leave = page.getByRole("dialog", {
    name: "Du har endringer som ikke er lagret",
  });
  await expect(leave).toBeVisible();
  await leave.getByRole("button", { name: "Bli her og lagre" }).click();
  await expect(leave).toBeHidden();
  await expect(page).toHaveURL(new RegExp(`/arrangementer/${seed.course}$`));
  await expect(page.getByLabel("Tittel")).toHaveValue(
    "Takkebakstkurs med Anita",
  );

  // The browser's back button too.
  await page.goBack();
  await expect(leave).toBeVisible();
  await leave.getByRole("button", { name: "Bli her og lagre" }).click();
  await expect(page.getByLabel("Tittel")).toHaveValue(
    "Takkebakstkurs med Anita",
  );

  // Closing the tab or reloading: the browser's own question.
  const dialog = page.waitForEvent("dialog");
  await page.close({ runBeforeUnload: true });
  const d = await dialog;
  expect(d.type()).toBe("beforeunload");
  await d.dismiss();
});

test("a server error lands next to its field, in words", async ({ page }) => {
  await page.goto("/admin/arrangementer/ny");
  await page.getByLabel("Lenke til billetter").fill("billetter.no/julemarked");
  await page.getByRole("button", { name: "Lagre kladd" }).click();

  const summary = page.locator("#feil-oppsummering");
  await expect(summary).toBeFocused();
  await expect(summary).toContainText("Dette må rettes før det kan lagres");
  const field = page.getByLabel("Lenke til billetter");
  await expect(field).toHaveAttribute("aria-invalid", "true");
  await expect(page.locator("#billettlenke-feil")).toContainText(
    "Må rettes: Lenken må være en hel nettadresse som starter med https://",
  );
  await expect(field).toHaveAccessibleDescription(/starter med https:\/\//);
  await summary.getByRole("button", { name: "Gå dit" }).click();
  await expect(field).toBeFocused();

  // Publishing an empty draft lists everything missing, each at its place.
  await field.fill("https://billetter.no/julemarked");
  await page.getByRole("button", { name: "Lagre kladd" }).click();
  await expect(page.getByText("Kladden er lagret.").first()).toBeVisible();
  await page.getByRole("button", { name: "Publiser" }).click();
  await expect(summary).toContainText(
    "Dette mangler før arrangementet kan publiseres",
  );
  await expect(page.locator("#tittel-feil")).toContainText("Skriv en tittel.");
  await expect(page.locator("#dato-feil")).toContainText(
    "Velg dagen arrangementet starter.",
  );
  await expect(page.locator("#hovedbilde-feil")).toContainText(
    "Velg et bilde.",
  );
  await noSideScroll(page);
});

test("a lost connection keeps what she typed, and a retry saves it", async ({
  page,
  seed,
}) => {
  await page.goto(`/admin/arrangementer/${seed.course}`);
  await page.getByLabel("Kort tekst").fill("Ny tekst skrevet på mobilen");
  await page.route(`**/admin/api/events/${seed.course}`, (route) =>
    route.request().method() === "PUT" ? route.abort() : route.continue(),
  );
  await page.getByRole("button", { name: "Lagre endringene" }).click();
  await expect(
    page
      .getByRole("alert")
      .filter({ hasText: "Fikk ikke kontakt med serveren" }),
  ).toBeVisible();
  await expect(page.getByLabel("Kort tekst")).toHaveValue(
    "Ny tekst skrevet på mobilen",
  );
  await page.unroute(`**/admin/api/events/${seed.course}`);
  await page.getByRole("button", { name: "Prøv igjen" }).click();
  await expect(
    page.getByText("Endringene er lagret og vises på nettsiden.").first(),
  ).toBeVisible();
});

test("two editors: hers is not lost and she chooses", async ({
  page,
  seed,
  request,
}) => {
  await page.goto(`/admin/arrangementer/${seed.course}`);
  await page.getByLabel("Pris").fill("Kr 1700,-");
  await request.post(`/__touch/${seed.course}`);
  await page.getByRole("button", { name: "Lagre endringene" }).click();
  const conflict = page.getByRole("dialog", {
    name: "Noen andre har endret arrangementet",
  });
  await expect(conflict).toBeVisible();
  // Escape chooses neither: her text stays, unsaved, and the page is hers
  // again (a dialog the browser closed but the page still held would leave
  // her stuck).
  await page.keyboard.press("Escape");
  await expect(conflict).toBeHidden();
  await expect(page.getByLabel("Pris")).toHaveValue("Kr 1700,-");
  await expect(page.getByText("Ikke lagret").first()).toBeVisible();
  await page.getByLabel("Pris").fill("Kr 1750,-");
  await page.getByRole("button", { name: "Lagre endringene" }).click();
  await expect(conflict).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(conflict).toBeHidden();
  await page.getByLabel("Pris").fill("Kr 1700,-");
  await page.getByRole("button", { name: "Lagre endringene" }).click();
  await conflict.getByRole("button", { name: "Lagre min versjon" }).click();
  await expect(page.getByText("Endringene er lagret").first()).toBeVisible();
  await expect(page.getByLabel("Pris")).toHaveValue("Kr 1700,-");
  const course = (
    await publicText(request, "/arrangementer/2026/takkebakstkurs")
  ).text;
  expect(course).toContain("Kr 1700,-");
});

test("two editors: «Lagre min versjon» publishes hers, not theirs", async ({
  page,
  seed,
  request,
}) => {
  const input = draftInput(seed, {
    title: "Høstmarked",
    summary: "Hennes tekst.",
  });
  const draft = await (
    await request.post("/admin/api/events", { data: input })
  ).json();
  await page.goto(`/admin/arrangementer/${draft.id}`);
  await expect(page.getByLabel("Kort tekst")).toHaveValue("Hennes tekst.");

  // Someone else saves their text while her page is open (the touch moves
  // the clock on, as stamps are whole seconds).
  await request.post(`/__touch/${draft.id}`);
  const seen = await (
    await request.get(`/admin/api/events/${draft.id}`)
  ).json();
  const other = await request.put(`/admin/api/events/${draft.id}`, {
    data: { ...input, updatedAt: seen.updatedAt, summary: "Deres tekst." },
  });
  expect(other.ok()).toBe(true);

  await page.getByRole("button", { name: "Publiser" }).click();
  const conflict = page.getByRole("dialog", {
    name: "Noen andre har endret arrangementet",
  });
  await conflict.getByRole("button", { name: "Lagre min versjon" }).click();
  await expect(
    page.getByText("Publisert! Arrangementet er nå på nettsiden.").first(),
  ).toBeVisible();
  const published = await publicText(request, "/arrangementer/2026/hostmarked");
  expect(published.status).toBe(200);
  expect(published.text).toContain("Hennes tekst.");
  expect(published.text).not.toContain("Deres tekst.");
});

test("someone without the grant is told so, plainly", async ({ page }) => {
  await page.route("**/admin/api/me", (route) =>
    route.fulfill({
      status: 403,
      contentType: "application/json",
      body: '{"errors":[{"code":"forbidden"}]}',
    }),
  );
  await page.goto("/admin/");
  await expect(
    page.getByRole("heading", { name: "Ingen tilgang" }),
  ).toBeVisible();
  await expect(page.getByRole("alert")).toHaveText(
    "Du har ikke tilgang til administrasjonen. Be Kristoffer om å gi deg tilgang.",
  );
});

test("uploads that fail say why and can be retried or removed", async ({
  page,
  seed,
}) => {
  await page.goto("/admin/arrangementer/ny");
  await choose(
    page,
    page.getByRole("button", { name: "Velg bilde" }),
    fixture("ikke-bilde.jpg"),
  );
  const row = page.locator('[data-upload="ikke-bilde.jpg"]');
  await expect(row).toContainText(
    "Dette er ikke et bilde nettsiden kan bruke.",
  );
  // A refused file leaves nothing to lose: no "unsaved", no question.
  await expect(page.getByText("Ikke lagret")).toHaveCount(0);
  await row.getByRole("button", { name: "Fjern" }).click();
  await expect(row).toBeHidden();

  // Over the limit: said at once, never sent, and no "try again".
  const sent = [];
  page.on("request", (r) => {
    if (r.url().endsWith("/admin/api/photos")) sent.push(r.url());
  });
  await choose(page, page.getByRole("button", { name: "Velg bilde" }), {
    name: "stort.jpg",
    mimeType: "image/jpeg",
    buffer: Buffer.alloc(41 * 1024 * 1024),
  });
  const big = page.locator('[data-upload="stort.jpg"]');
  await expect(big).toContainText("Filen er for stor (over 40 MB).");
  await expect(big.getByRole("button", { name: "Prøv igjen" })).toHaveCount(0);
  expect(sent).toEqual([]);
  await big.getByRole("button", { name: "Fjern" }).click();

  await page.route("**/admin/api/photos", (route) => route.abort());
  await choose(
    page,
    page.getByRole("button", { name: "Velg bilde" }),
    fixture("portrett.jpg"),
  );
  await expect(page.locator('[data-upload="portrett.jpg"]')).toContainText(
    "Mistet kontakten med serveren.",
  );
  // Kept in memory to send again, so leaving would lose it.
  await expect(page.getByText("Ikke lagret").first()).toBeVisible();
  await page.unroute("**/admin/api/photos");
  await page.getByRole("button", { name: "Prøv igjen" }).click();
  await expect(page.locator('[data-photo-status="ready"]')).toBeVisible({
    timeout: 60_000,
  });
});

test("a second main photo wins over a first still on its way", async ({
  page,
}) => {
  await page.goto("/admin/arrangementer/ny");
  let first = true;
  await page.route("**/admin/api/photos", async (route) => {
    if (first) {
      first = false;
      await new Promise((r) => setTimeout(r, 2000));
    }
    await route.continue().catch(() => {});
  });
  await choose(
    page,
    page.getByRole("button", { name: "Velg bilde" }),
    fixture("konsert.jpg"),
  );
  const second = page.waitForResponse(
    (r) => r.url().endsWith("/admin/api/photos") && r.status() < 300,
  );
  await choose(
    page,
    page.getByRole("button", { name: "Velg bilde" }),
    fixture("hage.jpg"),
  );
  const id = (await (await second).json()).id;
  await expect(page.locator(`#alt-${id}`)).toBeVisible();
  await expect(page.locator('[data-upload="konsert.jpg"]')).toHaveCount(0);
  // Past the moment the first answer would have come.
  await page.waitForTimeout(2500);
  await expect(page.locator(`#alt-${id}`)).toBeVisible();
  await expect(page.locator("input[id^=alt-]")).toHaveCount(1);
});

test("photographer: add in place, correct once, shown on every photo", async ({
  page,
  seed,
}) => {
  await page.goto("/admin/galleri");
  await expect(page.getByText("Bilde 2 av 2")).toBeVisible();
  const second = page.locator("[data-photo]").nth(1);
  await second.getByLabel("Fotograf (valgfritt)").selectOption("ny");
  const dlg = page.getByRole("dialog", { name: "Ny fotograf" });
  await expect(dlg.getByLabel("Navn")).toBeFocused();
  await dlg.getByLabel("Navn").fill("ola nordmann");
  await dlg.getByRole("button", { name: "Lagre fotografen" }).click();
  // Same name as an existing one: offer to use that one instead.
  await dlg.getByRole("button", { name: "Bruk «Ola Nordmann»" }).click();
  await expect(second.getByLabel("Fotograf (valgfritt)")).toHaveValue(/\d+/);
  await expect(
    second.getByText("Vises som «Foto: Ola Nordmann»"),
  ).toBeVisible();

  await second
    .getByRole("button", { name: "Rett navn eller lenke for Ola Nordmann" })
    .click();
  const fix = page.getByRole("dialog", { name: "Rett fotografen" });
  await fix
    .getByLabel("Lenke til fotografen (valgfritt)")
    .fill("instagram.com/ola");
  await fix.getByRole("button", { name: "Lagre fotografen" }).click();
  await expect(
    fix.getByText("Lenken må være en hel nettadresse"),
  ).toBeVisible();
  await fix.getByLabel("Navn").fill("Ola Nordmann Foto");
  await fix
    .getByLabel("Lenke til fotografen (valgfritt)")
    .fill("https://instagram.com/ola");
  await fix.getByRole("button", { name: "Lagre fotografen" }).click();
  await expect(fix).toBeHidden();
  await expect(
    page.getByText(
      "Vises som «Foto: Ola Nordmann Foto», med lenke til https://instagram.com/ola.",
    ),
  ).toHaveCount(2);
});

test("gallery: several uploads, order, alt text, and a photo in use", async ({
  page,
  seed,
  request,
}) => {
  await page.goto("/admin/galleri");
  await expect(page.getByText("Bilde 1 av 2")).toBeVisible();
  await noSideScroll(page);

  await choose(page, page.getByRole("button", { name: "+ Legg til bilder" }), [
    fixture("hage.jpg"),
    fixture("ikke-bilde.jpg"),
  ]);
  await expect(page.locator("#galleri-status")).toContainText(
    "1 bilde trenger hjelp",
  );
  await expect(page.getByText("Bilde 3 av 3")).toBeVisible();
  await expect(page.locator('[data-photo-status="ready"]')).toHaveCount(3, {
    timeout: 60_000,
  });

  const third = page.locator("[data-photo]").nth(2);
  const newId = Number(await third.getAttribute("data-photo"));
  await third.getByLabel("Beskriv bildet").fill("Hagen i juni");
  await third.getByLabel("Beskriv bildet").press("Tab");
  await expect(third.getByText("Lagret")).toBeVisible();
  expect((await publicText(request, "/galleri")).text).toContain(
    "Hagen i juni",
  );

  const saved = page.waitForRequest(
    (r) => r.url().endsWith("/admin/api/gallery") && r.method() === "PUT",
  );
  await page.getByRole("button", { name: "Flytt bilde 3 tidligere" }).click();
  expect((await saved).postDataJSON().photoIds).toEqual([
    seed.landskap,
    newId,
    seed.portrett,
  ]);
  await expect(
    page.getByRole("button", { name: "Flytt bilde 2 tidligere" }),
  ).toBeFocused();

  // The kursholder photo is used by an event: explain, and offer the gentle way.
  await page.getByRole("button", { name: "Slett bilde 3" }).click();
  const dlg = page.getByRole("dialog", { name: "Slette bildet?" });
  await dlg.getByRole("button", { name: "Ja, slett" }).click();
  await expect(
    dlg.getByText(
      "Dette bildet brukes også i «Takkebakstkurs», så det kan ikke slettes helt.",
    ),
  ).toBeVisible();
  await dlg.getByRole("button", { name: "Ta ut av galleriet" }).click();
  await expect(page.getByText("Bilde 2 av 2")).toBeVisible();
  const galleri = (await publicText(request, "/galleri")).text;
  expect(galleri).not.toContain("Kursholder ved kjøkkenbenken");
  expect(galleri.indexOf("Boder med julepynt")).toBeLessThan(
    galleri.indexOf("Hagen i juni"),
  );
});

test("the crop dialog works from the keyboard and gives focus back", async ({
  page,
  seed,
}) => {
  await page.goto(`/admin/arrangementer/${seed.course}`);
  const open = page.getByRole("button", { name: "Beskjær" });
  await open.focus();
  await page.keyboard.press("Enter");
  const crop = page.getByRole("dialog", { name: "Beskjær bildet" });
  await expect(crop.getByRole("button", { name: "Bruk" })).toBeFocused();
  // The page behind is inert: Tab cycles through the dialog, and past its
  // last control the browser's own toolbar (the body, seen from the page).
  for (let i = 0; i < 25; i++) {
    await page.keyboard.press("Tab");
    const where = await crop.evaluate(
      (d) =>
        d.contains(document.activeElement) ||
        document.activeElement === document.body,
    );
    expect(where, `focus left the dialog after ${i + 1} tabs`).toBe(true);
  }
  await page.keyboard.press("Escape");
  await expect(crop).toBeHidden();
  await expect(open).toBeFocused();
});

// The course's photo is 900 px wide: its banner is just sharp (the column
// is 896), one step smaller is not, and the floor of 800 stops Mindre.
test("the crop dialog says, as she goes, when a crop will look soft", async ({
  page,
  seed,
}, testInfo) => {
  // A short laptop screen: the line must be in sight without scrolling.
  const { width, height } = page.viewportSize();
  await page.setViewportSize({ width, height: Math.min(height, 720) });
  await page.goto(`/admin/arrangementer/${seed.course}`);
  await expect(page.locator("#bilde-uskarpt")).toHaveCount(0);
  await page.getByRole("button", { name: "Beskjær" }).click();
  const crop = page.getByRole("dialog", { name: "Beskjær bildet" });
  await crop.getByRole("button", { name: /Øverst på siden/ }).click();
  const line = crop.locator("#beskjaer-skarphet");
  await expect(line).toHaveAttribute("aria-live", "polite");
  await expect(line.locator("[data-verdict=sharp] p")).toHaveText(["Skarpt."]);

  await crop.getByRole("button", { name: "Mindre" }).click();
  await expect(line.locator("[data-verdict=soft]")).toBeVisible();
  // As she sees it when she drags the frame: the dialog at its top, not
  // scrolled down to the buttons as a click here does.
  await crop.evaluate((d) => (d.scrollTop = 0));
  await expect(line).toBeInViewport({ ratio: 1 });
  await page.screenshot({
    path: join(
      process.env.OUT || "out",
      `${testInfo.project.name}-uskarpt.png`,
    ),
  });
  await expect(line).toContainText(
    "Dette utsnittet blir uskarpt på nettsiden. Gjør rammen større, eller bruk et større bilde om du har det.",
  );
  await expect(line).toContainText("Be om originalen.");
  await expect(line).not.toContainText("kan ikke bli mindre");

  await crop.getByRole("button", { name: "Større" }).click();
  await expect(line.locator("[data-verdict=sharp] p")).toHaveText(["Skarpt."]);

  for (let i = 0; i < 3; i++)
    await crop.getByRole("button", { name: "Mindre" }).click();
  await expect(crop.locator("asp-cropper")).toHaveAttribute(
    "rect",
    /^\d+,\d+,800,450$/,
  );
  await expect(line).toContainText("Rammen kan ikke bli mindre");
  await expect(line).toContainText("uskarpt");
});

// Crops from before today's rules: unchanged, they are saved with the rest,
// and her text edits never touch them.
test("an old crop under the minimum survives the dialog and a save", async ({
  page,
  seed,
  request,
}) => {
  const old = { x: 98, y: 300, w: 704, h: 396 };
  expect(
    (
      await request.post(`/__crop/${seed.portrett}`, { data: { banner: old } })
    ).ok(),
  ).toBe(true);
  await page.goto(`/admin/arrangementer/${seed.course}`);
  await page.getByRole("button", { name: "Beskjær" }).click();
  const crop = page.getByRole("dialog", { name: "Beskjær bildet" });
  await crop.getByRole("button", { name: /Øverst på siden/ }).click();
  const line = crop.locator("#beskjaer-skarphet");
  await expect(line.locator("[data-verdict=soft]")).toBeVisible();
  await expect(line).toContainText("Rammen kan ikke bli mindre");
  // Mindre never grows it to the minimum.
  await crop.getByRole("button", { name: "Mindre" }).click();
  await expect(crop.locator("asp-cropper")).toHaveAttribute(
    "rect",
    "98,300,704,396",
  );

  const saved = page.waitForResponse((r) => r.url().endsWith("/crops"));
  await crop.getByRole("button", { name: "Bruk" }).click();
  expect((await saved).status()).toBe(200);
  await expect(crop).toBeHidden();

  await page.getByLabel("Pris").fill("Kr 1700,-");
  await page.getByRole("button", { name: "Lagre endringene" }).click();
  await expect(page.getByText("Endringene er lagret").first()).toBeVisible();

  const photo = await (
    await request.get(`/admin/api/photos/${seed.portrett}`)
  ).json();
  expect(photo.crops.banner).toEqual(old);
  const course = await publicText(
    request,
    "/arrangementer/2026/takkebakstkurs",
  );
  expect(course.status).toBe(200);
  expect(course.text).toContain("Kr 1700,-");
});

test("a small gallery photo is said to be shown smaller, calmly", async ({
  page,
  seed,
}) => {
  await page.goto("/admin/galleri");
  // portrett.jpg is 900 px wide, under the lightbox's 992; landskap is 1300.
  await expect(
    page.locator(`[data-photo="${seed.portrett}"] [data-verdict="info"] p`),
  ).toHaveText(
    "Bildet er litt lite, så det vises mindre når man åpner det stort. Det går fint.",
  );
  await expect(
    page.locator(`[data-photo="${seed.landskap}"] [data-verdict]`),
  ).toHaveCount(0);
});

test("gallery: quick moves and another admin's photo all end up saved", async ({
  page,
  request,
  seed,
}) => {
  await page.goto("/admin/galleri");
  await expect(page.getByText("Bilde 2 av 2")).toBeVisible();

  // Another admin adds a photo after her page loaded.
  const upload = await request.post("/admin/api/photos", {
    multipart: {
      file: {
        name: "hage.jpg",
        mimeType: "image/jpeg",
        buffer: readFileSync(fixture("hage.jpg")),
      },
    },
  });
  expect(upload.ok()).toBe(true);
  const theirs = (await upload.json()).id;
  expect(
    (
      await request.put("/admin/api/gallery", {
        data: { photoIds: [seed.landskap, seed.portrett, theirs] },
      })
    ).ok(),
  ).toBe(true);

  // Her first save is slow; her second move comes before its answer.
  let puts = 0;
  await page.route("**/admin/api/gallery", async (route) => {
    if (route.request().method() === "PUT" && puts++ === 0)
      await new Promise((r) => setTimeout(r, 1500));
    await route.continue();
  });
  await page.getByRole("button", { name: "Flytt bilde 2 tidligere" }).click();
  await page.getByRole("button", { name: "Flytt bilde 2 tidligere" }).click();

  const stored = async () =>
    (await (await request.get("/admin/api/gallery")).json()).photos.map(
      (p) => p.id,
    );
  await expect
    .poll(stored, { timeout: 15_000 })
    .toEqual([seed.landskap, seed.portrett, theirs]);
  await expect(page.getByText("Bilde 3 av 3")).toBeVisible();
  const shown = await page
    .locator("[data-photo]")
    .evaluateAll((els) => els.map((e) => Number(e.dataset.photo)));
  expect(shown).toEqual([seed.landskap, seed.portrett, theirs]);
});

// Holds requests to url (a glob) with the method for ms before they go on;
// answered resolves once the first answer has reached the page.
async function hold(page, url, ms, method = "POST") {
  let done;
  const answered = new Promise((r) => (done = r));
  await page.route(url, async (route) => {
    if (route.request().method() !== method) return route.fallback();
    await new Promise((r) => setTimeout(r, ms));
    await route.continue();
    done();
  });
  return { answered };
}

test("a late answer stays with the page that asked", async ({ page, seed }) => {
  await page.goto(`/admin/arrangementer/${seed.course}`);
  await expect(page.getByLabel("Tittel")).toHaveValue("Takkebakstkurs");
  const { answered: late } = await hold(page, "**/unpublish", 1500);
  await page.getByRole("button", { name: "Gjør om til kladd" }).click();
  await page
    .getByRole("link", { name: "Arrangementer", exact: true })
    .first()
    .click();
  await page.getByRole("link", { name: "Nytt arrangement" }).click();
  await expect(page).toHaveURL(/\/admin\/arrangementer\/ny$/);
  await late;
  // The answer would land within a frame or two of reaching the page.
  await page.waitForTimeout(300);
  await expect(page).toHaveURL(/\/admin\/arrangementer\/ny$/);
  await expect(page.getByLabel("Tittel")).toHaveValue("");
  await expect(page.getByText("Kladd – vises ikke på nettsiden")).toBeVisible();
  await expect(
    page.getByText("Arrangementet er gjort om til kladd"),
  ).toHaveCount(0);
});

test("opening the crops and pressing Bruk changes none of them", async ({
  page,
  request,
  seed,
}) => {
  // landskap is 1300 px wide: its stored share crop is 682 high, which a
  // round trip through the preview's pixels would make 683.
  const draft = await (
    await request.post("/admin/api/events", { data: draftInput(seed) })
  ).json();
  const stored = (
    await (await request.get(`/admin/api/photos/${seed.landskap}`)).json()
  ).crops;
  await page.goto(`/admin/arrangementer/${draft.id}`);
  await page.getByRole("button", { name: "Beskjær" }).click();
  const crop = page.getByRole("dialog", { name: "Beskjær bildet" });
  for (const kind of [/Øverst på siden/, /Når noen deler lenken/, /I listen/]) {
    await crop.getByRole("button", { name: kind }).click();
    // The cropper reports on load what it would report; give it the time.
    await page.waitForTimeout(500);
  }
  const put = page.waitForRequest(
    (r) => r.url().endsWith("/crops") && r.method() === "PUT",
  );
  await crop.getByRole("button", { name: "Bruk" }).click();
  expect((await put).postDataJSON()).toEqual(stored);
});

test("the mouse wheel over the photo leaves the frame alone", async ({
  page,
  seed,
}) => {
  test.skip(
    test.info().project.name === "phone-webkit",
    "Playwright has no mouse wheel in mobile WebKit",
  );
  await page.goto(`/admin/arrangementer/${seed.course}`);
  await page.getByRole("button", { name: "Beskjær" }).click();
  const crop = page.getByRole("dialog", { name: "Beskjær bildet" });
  const cropper = crop.locator("asp-cropper");
  await expect(cropper).toBeVisible();
  await page.waitForTimeout(500);
  const before = await cropper.getAttribute("rect");
  await cropper.hover();
  for (let i = 0; i < 3; i++) {
    await page.mouse.wheel(0, -300);
    // Cropper takes one wheel step per 50 ms.
    await page.waitForTimeout(100);
  }
  await page.waitForTimeout(500);
  await expect(cropper).toHaveAttribute("rect", before);
});

test("the preview stays where she scrolled it while she writes", async ({
  page,
  seed,
}) => {
  await page.goto(`/admin/arrangementer/${seed.course}`);
  const iframe = page.locator("#forhandsvisning iframe");
  await expect(iframe).toBeVisible();
  const frame = await (await iframe.elementHandle()).contentFrame();
  // Halfway down the course's page.
  await expect
    .poll(() => frame.evaluate(() => document.body.scrollHeight - innerHeight))
    .toBeGreaterThan(100);
  const y = await frame.evaluate(() => {
    scrollTo(0, Math.floor((document.body.scrollHeight - innerHeight) / 2));
    return scrollY;
  });

  await page
    .getByLabel("Kort tekst")
    .fill("Lær å bake lefser, flatbrød og lomper.");
  await expect(
    page
      .frameLocator("#forhandsvisning iframe")
      .getByText("og lomper.")
      .first(),
  ).toBeAttached();
  await expect.poll(() => frame.evaluate(() => scrollY)).toBe(y);
});

test("the page's address is on this site, not a fixed domain", async ({
  page,
  seed,
}) => {
  const host = new URL(process.env.BASE).host;
  await page.goto(`/admin/arrangementer/${seed.course}`);
  await expect(
    page.getByText(`${host}/arrangementer/2026/takkebakstkurs`, {
      exact: true,
    }),
  ).toBeVisible();
  await page.goto("/admin/arrangementer/ny");
  await page.getByLabel("Adresse til siden (valgfritt)").fill("sommerfest");
  await expect(page.locator("#adresse-hjelp")).toContainText(
    `${host}/arrangementer/<år>/sommerfest`,
  );
  await expect(page.getByText("aspargesgaarden.no")).toHaveCount(0);
});

// Text on the page whose colour, laid over what is behind it, is under
// WCAG AA: 4.5:1, or 3:1 for large text. Disabled controls are exempt.
async function lowContrast(page) {
  return page.evaluate(() => {
    const ctx = document
      .createElement("canvas")
      .getContext("2d", { willReadFrequently: true });
    const rgba = (css) => {
      ctx.clearRect(0, 0, 1, 1);
      ctx.fillStyle = css;
      ctx.fillRect(0, 0, 1, 1);
      const d = ctx.getImageData(0, 0, 1, 1).data;
      return [d[0], d[1], d[2], d[3] / 255];
    };
    const over = (top, under) =>
      under.map((u, i) => (i < 3 ? top[i] * top[3] + u * (1 - top[3]) : 1));
    const lum = ([r, g, b]) => {
      const ch = (v) => {
        v /= 255;
        return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
      };
      return 0.2126 * ch(r) + 0.7152 * ch(g) + 0.0722 * ch(b);
    };
    const backdrop = (el) => {
      const chain = [];
      for (let e = el; e; e = e.parentElement) chain.unshift(e);
      return chain.reduce(
        (bg, e) => over(rgba(getComputedStyle(e).backgroundColor), bg),
        [255, 255, 255, 1],
      );
    };
    const bad = [];
    for (const el of document.querySelectorAll("body *")) {
      const own = [...el.childNodes].some(
        (n) => n.nodeType === 3 && n.textContent.trim(),
      );
      if (!own || el.closest("[disabled], .sr-only, iframe")) continue;
      const r = el.getBoundingClientRect();
      if (!r.width || !r.height) continue;
      const cs = getComputedStyle(el);
      const bg = backdrop(el);
      const fg = over(rgba(cs.color), bg);
      const [hi, lo] = [lum(fg), lum(bg)].sort((a, b) => b - a);
      const ratio = (hi + 0.05) / (lo + 0.05);
      const size = parseFloat(cs.fontSize);
      const large =
        size >= 24 || (size >= 18.66 && Number(cs.fontWeight) >= 700);
      if (ratio < (large ? 3 : 4.5))
        bad.push(`${ratio.toFixed(2)} ${el.textContent.trim().slice(0, 50)}`);
    }
    return bad;
  });
}

test("every text in the admin is readable against what is behind it", async ({
  page,
  seed,
}) => {
  await page.goto("/admin/");
  await expect(
    page.getByRole("heading", { name: "Takkebakstkurs" }),
  ).toBeVisible();
  expect(await lowContrast(page)).toEqual([]);
  await page.goto("/admin/galleri");
  await expect(page.getByText("Bilde 2 av 2")).toBeVisible();
  expect(await lowContrast(page)).toEqual([]);
  await page.goto(`/admin/arrangementer/${seed.course}`);
  await expect(page.getByLabel("Tittel")).toHaveValue("Takkebakstkurs");
  expect(await lowContrast(page)).toEqual([]);
});

test("a photo the server cannot prepare says why", async ({
  page,
  request,
  seed,
}) => {
  // broken's original is missing, so every attempt fails, and trying
  // again cannot help: she is told to upload it again.
  const draft = await (
    await request.post("/admin/api/events", {
      data: draftInput(seed, {
        mainPhoto: { id: seed.broken, alt: "Låven", photographerId: null },
      }),
    })
  ).json();
  await request.post(`/admin/api/photos/${seed.broken}/prepare`);
  await expect
    .poll(async () => {
      const res = await request.get(
        `/admin/api/photos/status?ids=${seed.broken}`,
      );
      return (await res.json()).statuses[seed.broken].state;
    })
    .toBe("failed");

  await page.goto(`/admin/arrangementer/${draft.id}`);
  const photo = page.locator("#bilde");
  // The server names a missing original as such, or as its own failure
  // (internal), which trying again may cure: then it offers that, and the
  // retry fails again here.
  const failed = photo.locator('[data-photo-status="failed"]');
  await expect(failed).toContainText(
    /Den opprinnelige filen til bildet er borte fra serveren\. Last opp det samme bildet på nytt\.|Bildet kunne ikke klargjøres\. Trykk «Prøv igjen»\./,
  );
  const retry = photo.getByRole("button", { name: "Prøv igjen" });
  if (await retry.count()) {
    const retried = page.waitForRequest((r) => r.url().endsWith("/prepare"));
    await retry.click();
    await retried;
    await expect(failed).toBeVisible({ timeout: 30_000 });
  }
  await expect(photo.getByText("Ingen forhåndsvisning")).toBeVisible();
  await expect(photo.getByRole("button", { name: "Beskjær" })).toBeDisabled();

  // Publishing is never blocked by a photo's state.
  await page.getByRole("button", { name: "Publiser" }).click();
  await expect(
    page.getByText("Publisert! Arrangementet er nå på nettsiden.").first(),
  ).toBeVisible();

  // In the gallery it is counted among the photos that need help.
  await request.put("/admin/api/gallery", {
    data: { photoIds: [seed.landskap, seed.portrett, seed.broken] },
  });
  await page.goto("/admin/galleri");
  await expect(page.locator("#galleri-status")).toContainText(
    "1 bilde trenger hjelp",
  );
});

test("a description changed elsewhere is not undone unseen", async ({
  page,
  request,
  seed,
}) => {
  await page.goto(`/admin/arrangementer/${seed.course}`);
  const alt = page.locator(`#alt-${seed.portrett}`);
  await expect(alt).toHaveValue("Kursholder ved kjøkkenbenken");
  await alt.fill("Kursholderen viser deigen");
  // Versions are whole seconds: the gallery's edit must land in a later one.
  await page.waitForTimeout(1100);
  const theirs = await request.put(`/admin/api/photos/${seed.portrett}`, {
    data: { alt: "Anita ved benken", photographerId: null },
  });
  expect(theirs.ok()).toBe(true);

  await page.getByRole("button", { name: "Lagre endringene" }).click();
  const dialog = page.getByRole("dialog", {
    name: "Noen andre har endret et bilde",
  });
  await expect(dialog).toContainText("«Anita ved benken»");
  await dialog.getByRole("button", { name: "Bruk min" }).click();
  await expect(page.getByText("Endringene er lagret").first()).toBeVisible();
  const stored = await (
    await request.get(`/admin/api/photos/${seed.portrett}`)
  ).json();
  expect(stored.alt).toBe("Kursholderen viser deigen");
});
// Safari on an iPad never asks before a tab is closed, and may drop a tab
// she has left; a draft she has typed in is saved when the tab is hidden.
test("a draft is saved when she leaves the tab", async ({ page }) => {
  await page.goto("/admin/arrangementer/ny");
  await page.getByLabel("Tittel").fill("Sommerfest");
  const created = page.waitForResponse(
    (r) =>
      r.url().endsWith("/admin/api/events") && r.request().method() === "POST",
  );
  await page.evaluate(() => {
    Object.defineProperty(document, "visibilityState", {
      value: "hidden",
      configurable: true,
    });
    document.dispatchEvent(new Event("visibilitychange"));
  });
  expect((await created).status()).toBe(201);
  await expect(page).toHaveURL(/\/admin\/arrangementer\/\d+$/);
  await expect(
    page.getByText("Kladden er lagret. Den vises ikke på nettsiden.").first(),
  ).toBeVisible();
  await expect(page.getByText("Ikke lagret")).toHaveCount(0);
});

test("a draft's address follows its title until she chooses one", async ({
  page,
}) => {
  const host = new URL(process.env.BASE).host;
  await page.goto("/admin/arrangementer/ny");
  await page.getByLabel("Tittel").fill("Julemarkd");
  await page.getByLabel("Dato", { exact: true }).fill("2026-11-14");
  await page.getByRole("button", { name: "Lagre kladd" }).click();
  await expect(page.getByText("Kladden er lagret.").first()).toBeVisible();
  const field = page.getByLabel("Adresse til siden (valgfritt)");
  await expect(field).toHaveValue("");
  await expect(page.locator("#adresse-hjelp")).toContainText(
    `Lages av tittelen: ${host}/arrangementer/2026/julemarkd.`,
  );

  // The typo fixed: the address follows, and says so honestly meanwhile.
  await page.getByLabel("Tittel").fill("Julemarked");
  await expect(page.locator("#adresse-hjelp")).toContainText(
    "Lages av tittelen når du lagrer",
  );
  await page.getByRole("button", { name: "Lagre kladd" }).click();
  await expect(page.locator("#adresse-hjelp")).toContainText(
    `${host}/arrangementer/2026/julemarked.`,
  );

  // One she types stays, whatever the title.
  await field.fill("jul");
  await page.getByLabel("Tittel").fill("Julemarked på gården");
  await page.getByRole("button", { name: "Lagre kladd" }).click();
  await expect(page.getByText("Kladden er lagret.").first()).toBeVisible();
  await expect(field).toHaveValue("jul");
  await expect(page.locator("#adresse-hjelp")).toContainText(
    `${host}/arrangementer/2026/jul – tøm feltet`,
  );
});

test("a poster: chosen in its slot, whole in the preview and on the page, and shared", async ({
  page,
  request,
  seed,
}) => {
  const draft = await (
    await request.post("/admin/api/events", { data: draftInput(seed) })
  ).json();
  await page.goto(`/admin/arrangementer/${draft.id}`);
  const slot = page.locator("#plakat-seksjon");
  await choose(
    page,
    slot.getByRole("button", { name: "Velg plakat" }),
    fixture("konsert.jpg"),
  );
  await expect(slot.locator('[data-photo-status="ready"]')).toBeVisible({
    timeout: 60_000,
  });
  await slot
    .getByLabel("Hva står på plakaten?")
    .fill("Plakat: høstmarked 21. november");
  const frame = page.frameLocator("#forhandsvisning iframe");
  await expect(
    frame.locator('img[alt="Plakat: høstmarked 21. november"]'),
  ).toHaveCount(1);

  await page.getByRole("button", { name: "Publiser" }).click();
  await expect(
    page.getByText("Publisert! Arrangementet er nå på nettsiden.").first(),
  ).toBeVisible();
  const event = await (
    await request.get(`/admin/api/events/${draft.id}`)
  ).json();
  const sha = event.poster.previewUrl.split("/")[3];
  const ogImage = (html) =>
    /<meta content="([^"]+)" property="og:image">/.exec(html)?.[1];

  const live = (await publicText(request, event.path)).text;
  expect(live).toContain('alt="Plakat: høstmarked 21. november"');
  expect(ogImage(live)).toContain(`/${sha}/full-`);
  // The banner is still the main photo.
  expect(cropOf(live, "banner")).toBeTruthy();

  await slot.getByRole("button", { name: "Ta bort plakaten" }).click();
  await page.getByRole("button", { name: "Lagre endringene" }).click();
  await expect(page.getByText("Endringene er lagret").first()).toBeVisible();
  const without = (await publicText(request, event.path)).text;
  expect(without).not.toContain("Plakat: høstmarked");
  expect(ogImage(without)).not.toContain(sha);
});

test("a caption shows in the lightbox, and nothing where there is none", async ({
  page,
  seed,
}) => {
  await page.goto("/admin/galleri");
  const card = page.locator(`[data-photo="${seed.landskap}"]`);
  await card.getByLabel("Bildetekst (valgfritt)").fill("Julemarkedet i fjor");
  await card.getByLabel("Bildetekst (valgfritt)").press("Tab");
  await expect(card.getByText("Lagret")).toBeVisible();

  await page.goto("/galleri");
  await page.locator("[data-gallery] a[data-index='0']").click();
  const caption = page.locator("[role=dialog] figcaption");
  await expect(caption).toContainText("Julemarkedet i fjor");
  await expect(caption).not.toContainText("Boder med julepynt");
  await expect(page.locator("[role=dialog] [aria-live=polite]")).toContainText(
    "Boder med julepynt i låven",
  );
  // portrett has neither caption nor photographer: nothing under it.
  await page.getByRole("button", { name: "Neste bilde" }).click();
  await expect(caption).toHaveText("");
});
