# TODO

Left out on purpose. Each item: the fact, then the next action.

## Content for search and AI answers

Visible facts on the page count for more than metadata. The venue facts are
already in the structured data (`seo/`, `site/`); the pages do not say them
yet.

- [ ] `/bryllup`: weddings, the most searched use. Rooms and guest numbers
      (Låven 100, Fjøset 68), what is included, how to book. Code only, as
      the fixed pages are; add it to the sitemap and `llms.txt`. No prices.
- [ ] A practical-info page (`/praktisk-info`): address, parking, how to get
      there, capacity, accessibility, contact. The same facts as the
      `LocalBusiness`/`EventVenue` JSON-LD.
- [ ] Front page text: a short paragraph that says what the place is and
      where (Tjølling, Larvik), above the fold.
- [ ] An English summary page for visitors from abroad, linked from the
      footer, with `hreflang` on both sides.

## Off-site

- [ ] Google Business Profile: claim or verify, same name, address, phone
      and categories as the site; link to the site.
- [ ] Bing Places: the same, or import from Google.
- [ ] Ask guests for reviews (Google); answer them.
- [ ] Directory listings (venue and wedding directories, the municipality's
      and the region's tourism pages) with the same name, address and phone.

## Settings to look up

- [ ] Umami site id: find or create the site on umami.kradalby.no and set
      `services.asparges.umamiSiteId` at cutover (unset means no tracker).
- [ ] Font licence: Cardillac is a commercial font, served from the binary.
      Confirm the web licence covers self-hosting.

## Outside this repository

- [ ] The `kra` web library mounts `/debug`, `/who` and `/quitquitquit` on
      tsnet nodes without the capability check the admin has; any tailnet
      peer that reaches the node (monitoring) can call them. Any website can
      too, through a tailnet user's browser: a no-cors POST to
      `/quitquitquit` exits the service, and `/debug` is readable through
      DNS rebinding. Fix in `kra`: gate them like `HandleTSOnly` or behind a
      capability, with cross-origin protection and a check of its own Host.
- [ ] `kra`'s `Handle` mounts a handler on the local and the tsnet listener
      alike; there is no local-only registration. `adminFilesOnTailnet`
      (`cmd/asparges`) stands in by Tailscale address range. Fix in `kra`
      (a local-only `Handle`), then drop it.

## Hardening

- [ ] vips children run as the service user: a compromised decoder reaches
      the database, `originals/` and the tsnet state. Run them as a user of
      their own, or in a sandbox that sees only their input and output.
- [ ] The binary is built with cgo (`CGO_ENABLED=1`, the nixpkgs default),
      which rules out dropping ambient capabilities process-wide
      (`syscall.AllThreadsSyscall`); `images` drops them per child instead.
      Decide whether the build needs cgo.
- [ ] Move to a new repository with fresh history (this one carries the old
      site's photos in its history), then rename the Go module's GitHub
      repository to match `github.com/kradalby/asparges`.

## Not yet tested

- [ ] HTTPS on the real tailnet: the tsnet node's certificate
      (`https://asparges-beta.dalby.ts.net`). The VM test uses plain HTTP.
- [ ] Safari and iOS on a real device: the admin (photo upload from the
      camera roll, the cropper by touch) and the gallery's swipe. The browser
      suites use Playwright's WebKit, not Safari.
- [ ] The first deploy on `core.oracldn` (aarch64): the checklist in
      [docs/deploy.md](docs/deploy.md#7-verify--the-first-real-arm-deploy).

## Known limits

- [ ] A photo over about 34 times taller than wide (a long screenshot, a
      poster strip) is refused as `unsupported_type`: WebP and AVIF stop at
      16383 px a side, and even its 480 px wide size is taller. Give it a
      reason code and a text of its own if it ever happens.
- [ ] `images.Store.Status` loads its photo from the database again, though
      every caller in `server/` already has it, once per photo on
      `GET /admin/api/events`. Pass the photo in (`Status(images.Photo)`).
- [ ] A two-day market (start and end date, start and end time) reads as
      one event from the first day's start to the last day's end, in the
      date line and the JSON-LD; the real hours are the same each day. Decide
      what an end date with an end time means, then render it so.
- [ ] Crops are not redaction. Every built size of every stored photo, the
      uncropped one included, is public at its unguessable `/img/` address,
      drafts and unused photos too. A photo that must not be seen has to be
      deleted, not cropped.
- [ ] On iPad, unsaved edits to a published event are lost when the tab is
      closed; a draft is saved when the tab is hidden, a published event is
      not (saving would put the edit live). Keep the unsaved form in the
      browser and offer it back.
- [ ] Headings in `text-tre` on the tinted public background have a contrast
      of 3.0, under WCAG AA for text below large size. Darken the colour or
      the background where headings sit.
- [ ] A photo that fails before its sizes are built (original missing,
      unreadable) shows in the log and the admin, but in no metric:
      `asparges_images_build_errors_total` counts failed builds only. Count
      failed photos in a gauge and alert on it.

## After the cutover

- [ ] Remove `asparges import` and what only it uses: `cmd/asparges/import.go`
      and its test and `testdata/oldsite`, `site.ParseLegacyTime`,
      `FindOrCreatePhotographer` and the `EnsurePhotographer` query, the
      YAML module. Its users first: the gallery suite, `vm-basic`,
      `hardening-smoke` and the README's local loop need another way to
      fill a data directory (`serve --dev --e2e-fixtures` and its
      `/__reset`, or a script against the admin API).

## Content for the owners to check

- [ ] "Vårkonsert med Black & Lekker" (2026) is a draft: its only photo is
      393 px wide, under the 480 px minimum. Find a larger photo, then
      publish.
- [ ] Photo descriptions written during the import (every gallery photo
      that had none): read them, correct names and places.
- [ ] Photographer names and links, as imported from the old site.
- [ ] Event dates, times and "ca." notes the import moved into the text
      where the old free-text time was not a clock time.
