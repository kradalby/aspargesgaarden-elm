# Admin API

The JSON API the admin app (`elm/admin`) uses. It lives on the tsnet listener
only, behind the `kradalby.no/cap/asparges` capability (or loopback in
`--dev`). The app is the only client; it is written against this file, so a
change here means a change in both places.

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
- A route that takes JSON reads it only with `Content-Type: application/json`;
  anything else is `400 bad_request` (a cross-site form cannot send it
  without the browser asking first).
- Every request must name the admin node as its `Host`, and every write must
  come from the admin's own origin (`Sec-Fetch-Site`, `Origin`); the browser
  sends both for the app's own requests. See the refusals in the table.
- An unknown `/admin/api/…` route is `404 not_found` with the error body.
- Any route may also answer `500 internal`; the app treats it, an unexpected
  status or an unreadable body as "the server failed" and keeps everything
  the user typed.

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

| `code`                                                                     | Status      | `site` error           | `field`                              | Notes                                                                                                                                                                                        |
| -------------------------------------------------------------------------- | ----------- | ---------------------- | ------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bad_request`                                                              | 400         | —                      | — or a photo field                   | Body is not valid JSON, has the wrong types, or came without `Content-Type: application/json`. With `field` and `photoId`: that photo reference has an `updatedAt` that is no RFC 3339 time. |
| `forbidden`                                                                | 403         | —                      | —                                    | The caller has no identity on the tailnet, or no grant of the capability. The app says she lacks access and whom to ask.                                                                     |
| `wrong_host`                                                               | 403         | —                      | —                                    | The request's `Host` is not this node (in `--dev`: not loopback). Guards against a rebound DNS name.                                                                                         |
| `cross_origin`                                                             | 403         | —                      | —                                    | A write from another origin.                                                                                                                                                                 |
| `not_found`                                                                | 404         | `ErrNotFound`          | —                                    | Event or photo id unknown. On an event save, a photo id that does not exist is also `not_found` with its photo field (`mainPhoto`, `extraPhotos` or `poster`) and `photoId`.                 |
| `conflict`                                                                 | 409         | `ErrStale`             | — or a photo field                   | `updatedAt` precondition failed (also an empty or unreadable one), see [Two editors](#two-editors). With `field` and `photoId`: that photo changed since the form loaded it.                 |
| `invalid_date`                                                             | 422         | `ErrInvalidDate`       | `start`, `end` or `extraDates`       |                                                                                                                                                                                              |
| `invalid_time`                                                             | 422         | `ErrInvalidTime`       | `startTime` or `endTime`             | The server checks the two times one by one so it knows which.                                                                                                                                |
| `slug_invalid`                                                             | 422         | `ErrSlugInvalid`       | `slug`                               |                                                                                                                                                                                              |
| `slug_reserved`                                                            | 422         | `ErrSlugReserved`      | `slug`                               |                                                                                                                                                                                              |
| `slug_taken`                                                               | 422         | `ErrSlugTaken`         | `slug`                               |                                                                                                                                                                                              |
| `slug_frozen`                                                              | 422         | `ErrSlugFrozen`        | `slug`                               |                                                                                                                                                                                              |
| `end_before_start`                                                         | 422         | `ErrEndBeforeStart`    | `end` or `endTime`                   | `end` when the last day is before the first; `endTime` when a one-day event ends before it starts.                                                                                           |
| `extra_date`                                                               | 422         | `ErrExtraDate`         | `extraDates`                         |                                                                                                                                                                                              |
| `ticket_url`                                                               | 422         | `ErrTicketURL`         | `ticketUrl`                          |                                                                                                                                                                                              |
| `title_required`                                                           | 422         | `ErrTitleRequired`     | `title`                              |                                                                                                                                                                                              |
| `start_required`                                                           | 422         | `ErrStartRequired`     | `start`                              |                                                                                                                                                                                              |
| `main_photo_required`                                                      | 422         | `ErrMainPhotoRequired` | `mainPhoto`                          |                                                                                                                                                                                              |
| `alt_required`                                                             | 422         | `ErrAltRequired`       | a photo field, or `alt`              | With `photoId`. One entry per photo. `alt` on the photo route.                                                                                                                               |
| `duplicate_photo`                                                          | 422         | `ErrDuplicatePhoto`    | `extraPhotos`, `poster` or —         | With `photoId`, on its second use. No field for the gallery.                                                                                                                                 |
| `photographer_name_required`                                               | 422         | `ErrPhotographerName`  | `name`                               | Empty name on a photographer create or update.                                                                                                                                               |
| `photographer_name_taken`                                                  | 409         | `ErrPhotographerTaken` | `name`                               | Another photographer has that name. Body also has `existing`: that Photographer, so the app can pick it instead.                                                                             |
| `photographer_url`                                                         | 422         | `ErrPhotographerURL`   | `url`                                | A non-empty link that is not an `http`/`https` address with a host (same rule as `ticket_url`).                                                                                              |
| `photo_in_use`                                                             | 409         | `ErrPhotoInUse`        | —                                    | Body also has `usedBy`, see [Delete a photo](#delete-a-photo).                                                                                                                               |
| `crop_kind`                                                                | 422         | (images/)              | the kind                             |                                                                                                                                                                                              |
| `crop_outside`                                                             | 422         | (images/)              | the kind (`card`, `banner`, `share`) |                                                                                                                                                                                              |
| `crop_aspect`                                                              | 422         | (images/)              | the kind                             | `\|h − w/aspect\|` over max(2 px, 1 % of `w/aspect`).                                                                                                                                        |
| `crop_too_small`                                                           | 422         | (images/)              | the kind                             | Narrower than the kind's minimum, see [Crops](#crops).                                                                                                                                       |
| `too_large`, `unsupported_type`, `damaged`, `too_small`, `too_many_pixels` | 413/415/422 | (images/)              | —                                    | Upload only, see [Photos](#photos).                                                                                                                                                          |

Any other `code` is shown as a generic "something went wrong" message, so the
server may add codes without breaking the app.

The photo fields of an event are `mainPhoto`, `extraPhotos` and `poster`.

## Types

### Photo

```json
{
  "id": 12,
  "width": 4032,
  "height": 3024,
  "alt": "Låven pyntet til jul",
  "caption": "",
  "photographer": { "id": 3, "name": "Ola Nordmann", "url": "https://example.no" },
  "previewUrl": "/img/v<pipeline>/<sha256>/full-<crop>-1200.jpg",
  "thumbUrl": "/img/v<pipeline>/<sha256>/full-<crop>-480.jpg",
  "crops": {
    "card": { "x": 1012, "y": 0, "w": 2419, "h": 3024 },
    "banner": { "x": 0, "y": 378, "w": 4032, "h": 2268 },
    "share": { "x": 0, "y": 453, "w": 4032, "h": 2117 }
  },
  "quality": {
    "card": { "minWidth": 480, "sharpWidth": 768, "soft": false },
    "banner": { "minWidth": 800, "sharpWidth": 896, "soft": false },
    "share": { "minWidth": 1200, "sharpWidth": 1200, "soft": false },
    "full": { "minWidth": 480, "sharpWidth": 992, "soft": false }
  },
  "status": PhotoStatus,
  "updatedAt": "2026-11-10T09:12:44Z"
}
```

- `alt`: what the photo shows, read out to blind visitors and read by
  search engines; not shown under the photo. `caption`: optional plain text
  the lightbox shows under the photo; `""` for none, and then it shows no
  caption.
- `photographer`: the credited Photographer as stored now, or `null`.
- `width`/`height`: the **upright** size of the original, i.e. after EXIF
  autorotation. Crop rectangles use this frame.
- `previewUrl`: the server's own uncropped, upright JPEG, the widest of its
  sizes up to 1200 px (smaller for a smaller photo) — never the original. The
  crop editor draws on it; it exists once `status.previewReady` is `true`.
  `thumbUrl`: the same at 480 px wide, for lists; the upload builds both.
  Before `previewReady` the app shows a placeholder and requests neither.
- `crops`: always all three kinds. A kind that was never set, or that the
  photo is too small for (see [Crops](#crops)), holds the largest centred
  rectangle of its shape, so the app never computes one itself.
- `quality`: always all four kinds (`full` is the uncropped photo: in the
  gallery, below an event and as its poster). `minWidth` is the narrowest crop the server
  takes (see [Crops](#crops)); `sharpWidth` the widest box, in CSS pixels,
  the public pages draw the kind in, so the narrowest picture that is sharp
  on every screen (`images.Kind.SharpWidth`). Both are the same for every
  photo; the app takes them from here instead of knowing them. `soft`: what
  the pages draw for the kind now — its stored crop, or the whole photo where
  it has none (or one under `minWidth`) — is narrower than `sharpWidth`.
  `share` without a crop is never soft: the link preview then uses the
  banner. Soft never blocks anything; the app warns (for `full`: the photo is
  only shown smaller when opened large).
- `updatedAt`: the photo's version, a token like an event's. It changes with
  every change to the photo: its description, caption or credit, a crop, its
  photographer's name or link, an upload of the same bytes again, and its
  release from the last event or gallery that used it. Every answer that
  carries a Photo carries its current `updatedAt`.

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

- `state`: `preparing` (variants waiting for or in the build), `ready`
  (every variant the public pages need for this photo and its current crops
  exists), or `failed` (variants are missing and nothing is building them;
  see `reason`). `failed` stays until a build succeeds or the app asks again
  (`prepare`, a crop save, a re-upload). A crop change sends a `ready` photo
  back to `preparing` until the new set is built.
- `done`/`total`: variants built so far, of all expected (real counts; how
  many depends on the photo). `total` is `0` until known. The app shows
  "Klargjør bildet … 12 av 46" from them.
- `previewReady`: the preview and thumb sizes exist. The upload builds them,
  so the user can see and crop the photo while the rest is still being made.
- `reason`: `""` unless `failed`. What is wrong with the photo itself has
  the upload's code for it (see [Photos](#photos)): `damaged` (the file
  could not be read as a picture: truncated, corrupt), `too_small` (smaller
  than the smallest size the site uses, 480 px wide), and the rarer
  `unsupported_type`, `too_large` and `too_many_pixels`. `missing_original`:
  the stored upload is gone; uploading the same file again repairs it.
  `internal` is anything else (vips killed for memory or timed out, disk
  full): worth a retry. Any other code is shown like `internal`.
- A `failed` photo can still be referred to by events and the gallery; its
  missing sizes are missing on the public pages. Publishing is never blocked
  by a photo's status.
- No state is guaranteed to be seen: a photo whose sizes are still on disk
  (the same bytes uploaded again) is `ready` in the upload answer, and a
  poll may jump from `previewReady` to `ready`. The app announces where a
  photo is, never which step it passed.

### Photographer

```json
{ "id": 3, "name": "Ola Nordmann", "url": "https://example.no" }
```

A person to credit, shared by all their photos. `name` is required and
unique ignoring case and Unicode spelling (the server trims it and collapses inner whitespace);
`url` is optional (`""`), `http`/`https` only. Changing a photographer changes the credit on every photo that refers
to them.

### Event

```json
{
  "id": 7,
  "status": "draft",
  "slug": "julemarked",
  "slugFrozen": false,
  "slugFromTitle": true,
  "year": 2026,
  "path": "/arrangementer/2026/julemarked",
  "url": "https://aspargesgaarden.no/arrangementer/2026/julemarked",
  "dateText": "14. november kl. 12.00 – 15. november 2026 kl. 16.00 på Aspargesgården",
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
  "poster": null,
  "publishedAt": null,
  "createdAt": "2026-11-01T10:00:00Z",
  "updatedAt": "2026-11-10T09:12:44Z"
}
```

- `status`: `draft` or `published`. `slugFrozen` is `true` once the event has
  ever been published (`site.Event.SlugFrozen`). `slugFromTitle` is `true`
  while the slug is made from the title and follows it (see
  [EventInput](#eventinput)); `false` once someone chose it, by typing one
  or by copying an event.
- `year`/`path`/`url`: where the page is (or will be) on the public site.
  `year` is `0` while there is no start date, and `path`/`url` are `""` while
  there is no start date or no slug (no title yet). `url` is `path` on the
  configured `baseURL`.
- `dateText`: `site.Event.DateLine()` (`""` without a start date).
- `isOver`: `site.Event.IsOver(now)`; `false` for an event without a date.
- `mainPhoto` is a Photo or `null`; `extraPhotos` is a list of Photos in page
  order. `poster` is a Photo or `null`: an optional poster, shown whole in
  the page's text and used, uncropped, as the share image (`og:image`)
  instead of the main photo's share crop. The list and the banner keep the
  main photo. A photo is used once per event: the poster cannot also be
  the main or an extra photo (`duplicate_photo`, field `poster`).

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
  "mainPhoto": {
    "id": 12,
    "alt": "Låven pyntet til jul",
    "caption": "",
    "photographerId": 3,
    "updatedAt": "2026-11-10T09:12:44Z"
  },
  "extraPhotos": [
    {
      "id": 13,
      "alt": "Boder i fjøset",
      "caption": "Fjøset fylt av boder",
      "photographerId": null,
      "updatedAt": "2026-11-09T17:02:10Z"
    }
  ],
  "poster": null
}
```

- `updatedAt`: the value from the event as last loaded; ignored on create
  and preview (send `""`).
- `slug`: until first publish, `""` means "make it from the title", and the
  stored slug sent back unchanged while `slugFromTitle` keeps following the
  title: a corrected title corrects the address. Any other slug is one the
  user chose, and stays when the title changes until she empties the field.
  A copy's slug counts as chosen, so next year's market keeps its address.
  (`site.Apply`.) Ignored by the rules once `slugFrozen`; the app sends the
  frozen slug unchanged.
- `poster`: a photo reference like `mainPhoto`, or `null` (also when left
  out) for none.
- Photo references carry the alt text, the caption and the photographer
  (`photographerId`, or `null` for none), because the form edits them with
  the rest of the event. Saving the event stores each referenced photo's
  `alt`, `caption` and photographer (in the same transaction as the event).
  A reference without `caption` (or with `null`) keeps the stored one. An unknown
  `photographerId` is `404 not_found` with `field` and `photoId`. Crops are
  not part of this body; they are saved on their own (see [Crops](#crops)).
  Photographers themselves are created and corrected through their own
  routes, never through an event save.
- A photo reference's `updatedAt` is the Photo's `updatedAt` as the form
  last got it (from the event, an upload, a crop save or a conflict). Events
  share photos (a copy shows the original's), so a save whose `alt`,
  `caption` or `photographerId` differs from what is stored, for a photo changed since
  that version, is refused with `409 conflict` for that photo (see
  [Two editors](#two-editors)) instead of undoing the other change unseen.
  A photo sent as stored is never a conflict. Leaving `updatedAt` out (or
  `""`) skips the check, as older apps do; anything else that is not an
  RFC 3339 time is `400 bad_request` with `field` and `photoId`. Preview
  ignores it.
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
made by `site.Event.Copy`: everything but the dates, the same address
included (as a chosen slug), and not cancelled. A copy given a date in the
original's year is `slug_taken` until its address changes.

`DELETE /admin/api/events/{id}` → `204`, or `404`. Works for drafts and
published events; the app asks first. The photos stay: one that no other
event or the gallery uses is deleted a week later, unless an event or the
gallery takes it up again within that week.

Every successful write drops the render store (the public pages change).

### Two editors

Kristine and Kristoffer may have the same event open. Writes to an event
(`PUT`, `publish`, `unpublish`) carry the `updatedAt` the app last saw. If the
stored `updatedAt` differs, nothing is written and the answer is:

`409`

```json
{ "errors": [{ "code": "conflict" }], "current": Event, "photos": [] }
```

The app keeps the user's form, says that someone else has saved changes, and
offers two buttons: keep mine (resend with `current.updatedAt`) or take
theirs (load `current`). `copy` and `DELETE` need no precondition: a copy
reads the stored event, and a delete is confirmed by the user.

A save can also conflict on a photo it refers to (see
[EventInput](#eventinput)): the photo's description, caption or credit was changed
elsewhere (another event that shows it, or the gallery) since the form
loaded it. Each such photo is one entry with `field` and `photoId`, and
`photos` holds them as stored now:

```json
{
  "errors": [{ "code": "conflict", "field": "mainPhoto", "photoId": 12 }],
  "current": Event,
  "photos": [Photo]
}
```

Keep mine resends with each photo's `updatedAt` from `photos`; take theirs
takes `alt`, `caption` and `photographer` from there. `photos` is `[]` when only the
event itself conflicts. The event is checked first, so a save stale on both
reports the event, and the photo on the next try.

### Preview

`POST /admin/api/preview` with

```json
{ "view": "page", "id": 7, "event": EventInput }
```

→ `200 text/html; charset=utf-8`: a complete HTML document, rendered by the
same `pages/` code as the live site, from the unsaved input, never from the
database row.

- `view`: `page` = the event's own page; `list` = the `/arrangementer` page
  with this event in its place among the other published events (as if it
  were published; leaving it out of the list would show nothing useful).
- `id`: the saved event being edited; leave it out (or `0`) for a new one.
  With it, a published event keeps its frozen address, and the list shows the
  form in the saved event's place instead of next to it. The app sends it
  whenever the event has been saved. An unknown id is `404`.
- Rendering never fails on the publish rules: no title, no date and no photo
  render as empty text and a neutral placeholder. Only a body that is not
  valid JSON or a `view` other than `page` or `list` is a `400`, and an
  unknown photo or photographer id a `404`.
- Photos are drawn with their **stored** crops, so a crop is visible in the
  preview once it is saved (crops save on their own, see below).
- Asset URLs in the HTML are root-relative (`/static/…`, `/img/…`), and the
  admin listener serves `/static/` and `/img/` too, because the app shows the
  HTML in an `<iframe srcdoc sandbox="allow-same-origin">` on the admin
  origin. The server leaves every script out of it (analytics included: a
  preview is not a visit), and the sandbox would run none anyway.
- The response bypasses the render store; nothing is kept.

The app sends a preview about 0.6 s after the user stops typing and drops
answers to older requests.

### Photos

`POST /admin/api/photos` — `multipart/form-data` with one part `file` →
`201` Photo. The same bytes twice give the same photo (`200` then, with the
stored one). The server answers once the file is stored, its type sniffed,
its upright size read and its preview made, so the answer has
`previewReady: true`; the other sizes are built in the background, so its
`state` is normally `preparing` (or `ready`, if they are still on disk). A
new photo is used by nothing until an event save or a gallery change refers
to it. A photo nothing uses is deleted a week after it was last uploaded,
edited or taken out of use (by an event save, a gallery change or an event
delete), so a photo removed by mistake can be put back within the week.
A body that is not `multipart/form-data` with a `file` part is
`400 bad_request`. Upload errors, all without `field`:

| `code`             | Status | Meaning for the user                                    |
| ------------------ | ------ | ------------------------------------------------------- |
| `too_large`        | 413    | Over 40 MB. Pick a smaller copy.                        |
| `unsupported_type` | 415    | Not a JPEG, PNG, WebP, HEIC or AVIF photo by its bytes. |
| `damaged`          | 422    | Sniffed as a photo, but its size cannot be read.        |
| `too_small`        | 422    | Under 480 px wide upright.                              |
| `too_many_pixels`  | 422    | Over 100 million pixels; too big to decode.             |

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

Cheap: one photo lookup per id and file metadata, no crops or URLs. Unknown
ids are left out. Up to 100 ids per call; more, or an id that is not a
number, is `400 bad_request`. The app polls it every 2 s while at least one
photo it shows is `preparing` (or has `previewReady: false`), with every such
id in one call, and stops when none is. The URLs and crops in the Photo it
already has stay valid, so a status change needs no further request.

`POST /admin/api/photos/{id}/prepare` (no body) → `202` PhotoStatus. Starts
building again after a `failed`; a no-op for a photo that is `preparing` or
`ready`.

`PUT /admin/api/photos/{id}` with
`{ "alt": "", "caption": "", "photographerId": null }` → `200` Photo. Used
by the gallery editor; event forms send these with the event. Leaving
`caption` out keeps the stored one. An unknown `photographerId` is `404 not_found` with `field`
`photographerId` and `photoId`. An empty or all-whitespace `alt` is allowed
here (the gallery marks the photo instead), except on a photo a published
event shows: an edit never breaks a live page, so that is `422 alt_required`
with `field` `alt` and `photoId`.

#### Crops

`PUT /admin/api/photos/{id}/crops` with any subset of the kinds:

```json
{ "card": { "x": 1012, "y": 0, "w": 2419, "h": 3024 } }
```

→ `200` Photo. Rules (trust boundary): known kind, inside the upright photo,
`|h − w / aspect|` at most max(2 px, 1 % of `w / aspect`), and at least the
kind's minimum width: `card` 480, `banner` 800, `share` 1200. Errors carry
the kind as `field`. All kinds in the body are written or none.

A photo whose largest centred rectangle of a kind is narrower than that
minimum (every photo under 1200 px wide, for `share`) gets no crop of that
kind; its page uses another picture there (the share image falls back to the
banner). Such a kind's rectangle in the photo is the centred one, and
sending exactly that back is accepted and stores nothing, so the app may
send every kind as it got them; any other rectangle for it is
`crop_too_small`.
A kind's stored crop sent back exactly as the photo has it is accepted
without any check and left as it is, even under today's minimum (an old
import may hold one); a moved or resized one is checked as above. A crop
under `quality.sharpWidth` is accepted like any other: sharpness only warns,
and event saves and publishing never look at it.
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
route adds (after an upload), removes and reorders. A body without
`photoIds` is `400 bad_request` (it must not empty the gallery by accident);
`404 not_found` with `photoId` for an unknown id, `422 duplicate_photo` with
`photoId` for an id listed twice. No precondition: the last order saved
wins. A photo the saved list leaves out that nothing else uses is deleted a
week later, so the app sends one order at a time and reads the gallery just
before each: it keeps a photo someone else added since it last heard from
the server, and leaves out one someone else took out.

## The admin page

`pages.AdminHandler()` serves the document the app runs in (built with
elem-go in `pages/admin.go`); `server.Admin` mounts it at `/admin/`, on the
tailnet listener (in `--dev`, the local one). It answers `GET` and `HEAD` of
every path under it with the same document, so a deep link such as
`/admin/arrangementer/12` loads the app, which reads the address itself;
other methods get `405`. The pattern carries no method,
because `GET /admin/` would conflict with a method-less `/admin/api/`.

The document names `/static/admin.css` and `/static/admin.js` (with the
site's fonts) by their content-hashed URLs, so the admin listener also
serves `static.Handler()` under `/static/`. It has no inline script or
style, and the handler sends `ui.AdminCSP()`:

```
default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self';
font-src 'self'; connect-src 'self'; frame-src 'self'; object-src 'none';
base-uri 'none'; form-action 'none'; frame-ancestors 'none'
```

No `'unsafe-inline'` or `'unsafe-eval'`: Elm and Cropper.js set styles only
through the CSSOM, and the cropper's stylesheet is a constructed sheet. The
preview `iframe srcdoc` inherits this policy, so the preview HTML must load
everything from `'self'`, and must leave out the Umami script: it would be
refused (and a preview is not a visit).

## Open questions

1. **Seconds in `updatedAt`.** `events.updated_at` and `photos.updated_at`
   are stored in whole seconds, so two changes inside the same second pass
   the precondition. Accepted as rare for four admins; a `version` counter
   would close it.
2. **Crops take effect at once**, also on a published page (see Crops). The
   alternative, crops saved with the event, needs variants of a crop that is
   not yet stored, and the server builds only stored crops.
3. **Photographer delete.** `Store.DeletePhotographer` exists but the app
   has no screen for it (the owner asked for no management area), so there is
   no route. A misspelt duplicate is corrected in place instead.
4. **Gallery alt text is not enforced** by the server (many imported gallery
   photos have none). The gallery editor marks photos without a description
   instead. Enforce it once the import has been cleaned up?
