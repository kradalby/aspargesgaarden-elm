# Admin API

The JSON API the admin app (`elm/admin`) uses. It lives on the tsnet listener
only, behind the `kradalby.no/cap/asparges` capability (or loopback in
`--dev`). The app is the only client; it is written against this file, and
`e2e/fake-server.mjs` implements the same contract for the browser tests, so a
change here means a change in all three places.

## Conventions

- Base path `/admin/api/`. Request and response bodies are JSON
  (`Content-Type: application/json; charset=utf-8`) unless stated otherwise.
- Keys are camelCase. Every key listed is always present in responses; an
  empty value is `""`, `[]`, `false` or `null` as shown, never left out.
- Dates are `YYYY-MM-DD` (Oslo calendar day), clock times `HH:MM` (24 h). An
  unset date or time is `""`.
- Timestamps are RFC 3339 in UTC, e.g. `2026-11-10T09:12:44Z`. The app treats
  `updatedAt` as an opaque token and only sends it back.
- Status codes: `200` with a body, `201` for a created resource (body = the
  resource), `204` without a body. Errors use the shape below.
- No caching: responses carry `Cache-Control: no-store`.
- Every route also answers `403` (no capability) and `500`; the app treats an
  unexpected status or an unreadable body as "the server failed" and keeps
  everything the user typed.

## Errors

Any `4xx`/`5xx` from `/admin/api/` has this body:

```json
{
  "errors": [
    { "code": "title_required", "field": "title" },
    { "code": "alt_required", "field": "mainPhoto", "photoId": 12 }
  ]
}
```

- `errors` has one or more entries. A failed save reports **every** rule it
  breaks, not just the first (the `site` package already joins them).
- `code` is required. `field` and `photoId` are present only when stated in
  the table; otherwise they are absent. The app places a message next to
  `field`; an error without a field it knows goes in the list at the top of
  the form.
- The app owns the Norwegian text, so the server never sends user-facing
  text. A server may add `"detail": "<english>"` for logs; the app ignores it.

| `code`                                                  | Status      | `site` error           | `field`                              | Notes                                                                                                                                                      |
| ------------------------------------------------------- | ----------- | ---------------------- | ------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bad_request`                                           | 400         | —                      | —                                    | Body is not valid JSON or has the wrong types.                                                                                                             |
| `not_found`                                             | 404         | `ErrNotFound`          | —                                    | Event or photo id unknown. On an event save, a photo id that does not exist is also `not_found` with `field` = `mainPhoto` or `extraPhotos` and `photoId`. |
| `conflict`                                              | 409         | —                      | —                                    | `updatedAt` precondition failed, see [Two editors](#two-editors).                                                                                          |
| `invalid_date`                                          | 422         | `ErrInvalidDate`       | `start`, `end` or `extraDates`       |                                                                                                                                                            |
| `invalid_time`                                          | 422         | `ErrInvalidTime`       | `startTime` or `endTime`             | The server checks the two times one by one so it knows which.                                                                                              |
| `slug_invalid`                                          | 422         | `ErrSlugInvalid`       | `slug`                               |                                                                                                                                                            |
| `slug_reserved`                                         | 422         | `ErrSlugReserved`      | `slug`                               |                                                                                                                                                            |
| `slug_taken`                                            | 422         | `ErrSlugTaken`         | `slug`                               |                                                                                                                                                            |
| `slug_frozen`                                           | 422         | `ErrSlugFrozen`        | `slug`                               |                                                                                                                                                            |
| `end_before_start`                                      | 422         | `ErrEndBeforeStart`    | `end` or `endTime`                   | `end` when the last day is before the first; `endTime` when a one-day event ends before it starts.                                                         |
| `extra_date`                                            | 422         | `ErrExtraDate`         | `extraDates`                         |                                                                                                                                                            |
| `ticket_url`                                            | 422         | `ErrTicketURL`         | `ticketUrl`                          |                                                                                                                                                            |
| `title_required`                                        | 422         | `ErrTitleRequired`     | `title`                              |                                                                                                                                                            |
| `start_required`                                        | 422         | `ErrStartRequired`     | `start`                              |                                                                                                                                                            |
| `main_photo_required`                                   | 422         | `ErrMainPhotoRequired` | `mainPhoto`                          |                                                                                                                                                            |
| `alt_required`                                          | 422         | `ErrAltRequired`       | `mainPhoto` or `extraPhotos`         | With `photoId`. One entry per photo.                                                                                                                       |
| `duplicate_photo`                                       | 422         | `ErrDuplicatePhoto`    | `mainPhoto`, `extraPhotos` or —      | With `photoId`. No field for the gallery.                                                                                                                  |
| `photographer_name_required`                            | 422         | `ErrPhotographerName`  | `name`                               | Empty name on a photographer create or update.                                                                                                             |
| `photographer_name_taken`                               | 409         | `ErrPhotographerTaken` | `name`                               | Another photographer has that name. Body also has `existing`: that Photographer, so the app can pick it instead.                                           |
| `photographer_url`                                      | 422         | `ErrPhotographerURL`   | `url`                                | A non-empty link that is not an `http`/`https` address with a host (same rule as `ticket_url`).                                                            |
| `photo_in_use`                                          | 409         | `ErrPhotoInUse`        | —                                    | Body also has `usedBy`, see [Delete a photo](#delete-a-photo).                                                                                             |
| `crop_kind`                                             | 422         | `ErrCropKind`          | the kind                             |                                                                                                                                                            |
| `crop_outside`                                          | 422         | `ErrCropOutside`       | the kind (`card`, `banner`, `share`) |                                                                                                                                                            |
| `crop_aspect`                                           | 422         | (images/)              | the kind                             | `\|w/aspect − h\| > 1`.                                                                                                                                    |
| `crop_too_small`                                        | 422         | (images/)              | the kind                             | Smaller than the kind's smallest output width.                                                                                                             |
| `too_large`, `unsupported_type`, `damaged`, `too_small` | 413/415/422 | (images/)              | —                                    | Upload only, see [Photos](#photos).                                                                                                                        |

Any other `code` is shown as a generic "something went wrong" message, so the
server may add codes without breaking the app.

## Types

### Photo

```json
{
  "id": 12,
  "width": 4032,
  "height": 3024,
  "alt": "Låven pyntet til jul",
  "photographer": { "id": 3, "name": "Ola Nordmann", "url": "https://example.no" },
  "previewUrl": "/img/v1/<sha256>/full-00000000-1600.jpg",
  "thumbUrl": "/img/v1/<sha256>/full-00000000-480.jpg",
  "crops": {
    "card": { "x": 1012, "y": 0, "w": 2419, "h": 3024 },
    "banner": { "x": 0, "y": 378, "w": 4032, "h": 2268 },
    "share": { "x": 0, "y": 453, "w": 4032, "h": 2117 }
  },
  "status": PhotoStatus
}
```

- `photographer`: the credited Photographer as stored now, or `null`.
- `width`/`height`: the **upright** size of the original, i.e. after EXIF
  autorotation. Crop rectangles use this frame.
- `previewUrl`: the server's own uncropped, upright JPEG, long side at most
  1600 px — never the original. The crop editor draws on it. `thumbUrl`: the
  same at 480 px wide, for lists. Both load once `status.previewReady` is
  `true`; before that the app shows a placeholder and does not request them.
- `crops`: always all three kinds. A kind that was never set holds the
  centred default the server uses, so the app never computes defaults itself.

### PhotoStatus

How far the server has come with the photo's sizes. Designed for cheap
polling (see [Photo status](#photo-status)).

```json
{
  "state": "preparing",
  "done": 12,
  "total": 46,
  "previewReady": true,
  "reason": ""
}
```

- `state`: `preparing` (variants still being built), `ready` (every variant
  the public pages need for this photo and its current crops exists), or
  `failed` (the server gave up; see `reason`). A crop change sends a `ready`
  photo back to `preparing` until the new set is built.
- `done`/`total`: variants built so far, of all expected. `total` is `0` until
  known. The app shows "Klargjør bildet … 12 av 46" from them.
- `previewReady`: the preview and thumb sizes exist. Built first, so the user
  can see and crop the photo while the rest is still being made.
- `reason`: `""` unless `failed`. Codes the app explains:
  - `damaged`: the file could not be read as a picture (truncated, corrupt).
  - `too_small`: smaller than the smallest size the site uses (480 px wide).
  - `internal`: anything else (vips crashed, timed out, disk full). Worth a
    retry.
    Any other code is shown like `internal`.
- A `failed` photo can still be referred to by events and the gallery; the
  public pages fall back to building on request. Publishing is never blocked
  by a photo's status.

### Photographer

```json
{ "id": 3, "name": "Ola Nordmann", "url": "https://example.no" }
```

A person to credit, shared by all their photos. `name` is required and
unique ignoring case (the server trims it and collapses inner whitespace);
`url` is optional (`""`), `http`/`https` only. Changing a photographer changes the credit on every photo that refers
to them.

### Event

```json
{
  "id": 7,
  "status": "draft",
  "slug": "julemarked",
  "slugFrozen": false,
  "year": 2026,
  "path": "/arrangementer/2026/julemarked",
  "url": "https://aspargesgaarden.no/arrangementer/2026/julemarked",
  "dateText": "14. november 2026 kl 12:00 - 15. november 2026 kl 16:00 på Aspargesgården",
  "isOver": false,
  "title": "Julemarked 2026",
  "summary": "Velkommen til årets julemarked i Låven.",
  "body": "## Når og hvor\n\n14. og 15. november i Låven.",
  "start": "2026-11-14",
  "startTime": "12:00",
  "end": "2026-11-15",
  "endTime": "16:00",
  "extraDates": [],
  "priceText": "",
  "ticketUrl": "",
  "showInMenu": true,
  "menuLabel": "Julemarked",
  "cancelled": false,
  "mainPhoto": { "...": "Photo" },
  "extraPhotos": [],
  "publishedAt": null,
  "createdAt": "2026-11-01T10:00:00Z",
  "updatedAt": "2026-11-10T09:12:44Z"
}
```

- `status`: `draft` or `published`. `slugFrozen` is `true` once the event has
  ever been published (`site.Event.SlugFrozen`).
- `year`/`path`/`url`: where the page is (or will be) on the public site.
  `year` is `0` and `path`/`url` are `""` while there is no start date. `url`
  is `path` on the configured `baseURL`.
- `dateText`: `site.Event.DateLine()` (`""` without a start date).
- `isOver`: `site.Event.IsOver(now)`.
- `mainPhoto` is a Photo or `null`; `extraPhotos` is a list of Photos in page
  order.

### EventInput

The body of a create, update and preview. The editable fields of Event, with
photos as references:

```json
{
  "updatedAt": "2026-11-10T09:12:44Z",
  "slug": "",
  "title": "Julemarked 2026",
  "summary": "",
  "body": "",
  "start": "2026-11-14",
  "startTime": "12:00",
  "end": "",
  "endTime": "",
  "extraDates": [],
  "priceText": "",
  "ticketUrl": "",
  "showInMenu": false,
  "menuLabel": "",
  "cancelled": false,
  "mainPhoto": { "id": 12, "alt": "Låven pyntet til jul", "photographerId": 3 },
  "extraPhotos": [{ "id": 13, "alt": "Boder i fjøset", "photographerId": null }]
}
```

- `updatedAt`: the value from the event as last loaded; ignored on create
  and preview (send `""`).
- `slug`: `""` means "make it from the title" (`site.Revise`). Ignored by the
  rules once `slugFrozen`; the app sends the frozen slug unchanged.
- Photo references carry the alt text and the photographer (`photographerId`,
  or `null` for none), because the form edits them with the rest of the
  event. Saving the event stores each referenced photo's `alt` and
  photographer (in the same transaction as the event). An unknown
  `photographerId` is `404 not_found` with `field` and `photoId`. Crops are
  not part of this body; they are saved on their own (see [Crops](#crops)).
  Photographers themselves are created and corrected through their own
  routes, never through an event save.
- Text fields are stored as sent, except that the server trims surrounding
  whitespace from `title`, `slug`, `ticketUrl`, `menuLabel` and `priceText`.

## Routes

### Who am I

`GET /admin/api/me` → `200`

```json
{ "name": "Kristine", "siteUrl": "https://aspargesgaarden.no" }
```

`name` is the Tailscale `WhoIs` display name (`UserProfile.DisplayName`,
falling back to the login name). In `--dev` it is `"Utvikler"`. `siteUrl` is
`baseURL` without a trailing slash; the app links "Se nettsiden" to it.

### Events

`GET /admin/api/events` → `200`

```json
{ "upcoming": [Event], "drafts": [Event], "past": [Event] }
```

- `drafts`: every event with status `draft`, newest `updatedAt` first.
- `upcoming` and `past`: the published events split by `site.Split` with the
  server's clock, in its order.

`GET /admin/api/events/{id}` → `200` Event, or `404`.

`POST /admin/api/events` with EventInput → `201` Event (a new draft). `422`
with the rule errors that apply to any stored event (`site.Event.Validate`
for a draft: bad slug, bad date or time, end before start, bad extra date,
bad ticket link, duplicate photo). A draft may lack a title, date and photo.

`PUT /admin/api/events/{id}` with EventInput → `200` Event. Same rules; a
published event is also checked with `Publishable`, so an edit can never
break a live page. `409 conflict` on a stale `updatedAt`.

`POST /admin/api/events/{id}/publish` with `{ "updatedAt": "..." }` → `200`
Event. `422` lists what is missing (`title_required`, `start_required`,
`main_photo_required`, `alt_required` per photo, `slug_invalid`). `409
conflict` on a stale `updatedAt`. The app saves first when the form has
changes, then publishes with the `updatedAt` from that save.

`POST /admin/api/events/{id}/unpublish` with `{ "updatedAt": "..." }` → `200`
Event (now `draft`; the address stays frozen). `409 conflict` on a stale
`updatedAt`.

`POST /admin/api/events/{id}/copy` (no body) → `201` Event: the new draft
made by `site.Event.Copy` (same text, photos and menu setting; no dates).

`DELETE /admin/api/events/{id}` → `204`, or `404`. Works for drafts and
published events; the app asks first. The photos stay.

Every successful write drops the render store (the public pages change).

### Two editors

Kristine and Kristoffer may have the same event open. Writes to an event
(`PUT`, `publish`, `unpublish`) carry the `updatedAt` the app last saw. If the
stored `updatedAt` differs, nothing is written and the answer is:

`409`

```json
{ "errors": [{ "code": "conflict" }], "current": Event }
```

The app keeps the user's form, says that someone else has saved changes, and
offers two buttons: keep mine (resend with `current.updatedAt`) or take
theirs (load `current`). `copy` and `DELETE` need no precondition: a copy
reads the stored event, and a delete is confirmed by the user.

### Preview

`POST /admin/api/preview` with

```json
{ "view": "page", "event": EventInput }
```

→ `200 text/html; charset=utf-8`: a complete HTML document, rendered by the
same `pages/` code as the live site, from the unsaved input, never from the
database row.

- `view`: `page` = the event's own page; `list` = the `/arrangementer` page
  with this event in its place among the other published events (as if it
  were published; leaving it out of the list would show nothing useful).
- Rendering never fails on the publish rules: no title, no date and no photo
  render as empty text and a neutral placeholder. Only a body that is not
  valid JSON is a `400`, and an unknown photo id a `404`.
- Photos are drawn with their **stored** crops, so a crop is visible in the
  preview once it is saved (crops save on their own, see below).
- Asset URLs in the HTML are root-relative (`/static/…`, `/img/…`), and the
  admin listener serves `/static/` and `/img/` too, because the app shows the
  HTML in an `<iframe srcdoc sandbox="allow-same-origin">` on the admin
  origin. No scripts are needed or run in it.
- The response bypasses the render store; nothing is kept.

The app sends a preview about 0.6 s after the user stops typing and drops
answers to older requests.

### Photos

`POST /admin/api/photos` — `multipart/form-data` with one part `file` →
`201` Photo. The same bytes twice give the same photo (`200` then, with the
stored one). The server answers once the file is stored, its type sniffed and
its upright size read; building the sizes starts in the background, so the
answer's `status` is normally `preparing` with `previewReady: false`. The new
photo is used by nothing until an event save or a gallery change refers to
it. Upload errors, all without `field`:

| `code`             | Status | Meaning for the user                              |
| ------------------ | ------ | ------------------------------------------------- |
| `too_large`        | 413    | Over 40 MB. Pick a smaller copy.                  |
| `unsupported_type` | 415    | Not a JPEG, PNG, WebP or HEIC photo by its bytes. |
| `damaged`          | 422    | Sniffed as a photo, but its size cannot be read.  |
| `too_small`        | 422    | Under 480 px wide upright.                        |

The app keeps the chosen file in memory, so after a lost connection (no
answer at all) "Prøv igjen" sends it again and nothing typed is lost. The
user can cancel an upload in flight; the server must cope with the aborted
request (discard the partial file in `tmp/`).

`GET /admin/api/photos/{id}` → `200` Photo.

#### Photo status

`GET /admin/api/photos/status?ids=12,13,14` → `200`

```json
{ "statuses": { "12": PhotoStatus, "13": PhotoStatus } }
```

Cheap: no crops, no URLs, no database joins beyond the ids. Unknown ids are
left out. Up to 100 ids per call. The app polls it every 2 s while at least one
photo it shows is `preparing` (or has `previewReady: false`), with every such
id in one call, and stops when none is. The URLs and crops in the Photo it
already has stay valid, so a status change needs no further request.

`POST /admin/api/photos/{id}/prepare` (no body) → `202` PhotoStatus. Starts
building again after a `failed`; a no-op for a photo that is `preparing` or
`ready`.

`PUT /admin/api/photos/{id}` with `{ "alt": "", "photographerId": null }` →
`200` Photo. Used by the gallery editor; event forms send these with the
event. An unknown `photographerId` is `404 not_found` with `field`
`photographerId`. An empty `alt` is allowed here (the gallery marks the
photo instead); the publish rules only apply to events.

#### Crops

`PUT /admin/api/photos/{id}/crops` with any subset of the kinds:

```json
{ "card": { "x": 1012, "y": 0, "w": 2419, "h": 3024 } }
```

→ `200` Photo. Rules (trust boundary): known kind, inside the upright photo,
`|w / aspect − h| ≤ 1`, at least the kind's smallest output width. Errors
carry the kind as `field`. All kinds in the body are written or none.
A crop belongs to the photo, so it takes effect on every page that shows the
photo as soon as it is saved, published or not. The app says so in the crop
dialog.

Aspects: `card` 4:5, `banner` 16:9, `share` 1200:630.

#### Delete a photo

`DELETE /admin/api/photos/{id}` → `204`. If the photo is in the gallery and
used by no event, it is taken out of the gallery and deleted in one
transaction. If any event uses it:

`409`

```json
{
  "errors": [{ "code": "photo_in_use" }],
  "usedBy": [{ "id": 7, "title": "Julemarked 2026" }]
}
```

### Photographers

`GET /admin/api/photographers` → `200` `{ "photographers": [Photographer] }`,
sorted by name (Norwegian collation is not required; case-insensitive is
enough). The app loads it once per page for the picker.

`POST /admin/api/photographers` with `{ "name": "", "url": "" }` → `201`
Photographer. Errors: `photographer_name_required`, `photographer_url`,
`photographer_name_taken` (with `existing`).

`PUT /admin/api/photographers/{id}` with `{ "name": "", "url": "" }` → `200`
Photographer. Same errors, plus `404`. Drops the render store, since every
page with one of their photos changes.

There is no delete: a photographer nobody refers to is harmless, and the app
has no screen for it.

### Gallery

`GET /admin/api/gallery` → `200`

```json
{ "photos": [Photo] }
```

in display order.

`PUT /admin/api/gallery` with `{ "photoIds": [12, 13, 9] }` → `200`
`{ "photos": [Photo] }`. Replaces the whole gallery in that order; this one
route adds (after an upload), removes and reorders. `404 not_found` with
`photoId` for an unknown id, `422 duplicate_photo` with `photoId` for an id
listed twice. No precondition: the last order saved wins, which is harmless
for a list of photos.

## The admin page

Every address under `/admin/` that is not `/admin/api/` or `/admin/static/`
answers `200` with the same HTML shell (`elm/admin/index.html`); the Elm app
reads the address itself. The shell loads `/admin/static/admin.css`,
`/admin/static/app.js` (Elm) and `/admin/static/elements.js` (custom elements
and start-up), in that order, all `defer`. It has no inline script or style.

The app works under this policy, which the fake server sends in the browser
tests:

```
default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self';
font-src 'self'; connect-src 'self'; frame-src 'self'; object-src 'none';
base-uri 'none'; form-action 'none'; frame-ancestors 'none'
```

No `'unsafe-inline'` or `'unsafe-eval'`: Elm and Cropper.js set styles only
through the CSSOM, and the cropper's stylesheet is a constructed sheet. The
preview `iframe srcdoc` inherits this policy, so the preview HTML must load
everything from `'self'` too (the public pages already do).

## Open questions

1. **Seconds in `updatedAt`.** `events.updated_at` is stored in whole
   seconds, so two saves of one event inside the same second pass the
   precondition. Accepted as rare for four admins; a `version` counter would
   close it.
2. **Orphan photos.** A photo uploaded into a form that is never saved, or
   replaced as main photo, keeps its `photos` row with no user. `Reconcile`
   only removes originals without a row. Should a nightly job delete photo
   rows that no event and no gallery has used for 7 days?
3. **Crops take effect at once**, also on a published page (see Crops). The
   alternative, crops saved with the event, needs `/img/` to serve variants of
   a crop that is not yet stored, which the plan's strict variant rule
   forbids. Fine as is?
4. **Preview path.** The plan named `/admin/preview`; this puts it under
   `/admin/api/preview` so one prefix carries the auth check and the error
   shape. Either works for the app if decided before the Go side is written.
5. **`list` preview of a draft** places the draft among the published events
   by its date. Should drafts without a date show at the top instead? The app
   does not care; the Go side picks.
6. **Photographer delete.** `Store.DeletePhotographer` exists but the app
   has no screen for it (the owner asked for no management area), so there is
   no route. A misspelt duplicate is corrected in place instead.
7. **Gallery alt text is not enforced** by the server (many imported gallery
   photos have none today). The gallery editor marks photos without a
   description instead. Enforce it once the import has been cleaned up?
