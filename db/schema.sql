-- asparges schema. Source of truth for squibble: the live database is
-- migrated to match this file exactly (see db.go). Rules that need more than
-- one row live in site/; the checks here are the ones a bad write must never
-- get past, whatever code path it comes from.
--
-- Dates are TEXT 'YYYY-MM-DD' (civil dates in Europe/Oslo, '' for none) and
-- times TEXT 'HH:MM' ('' for none). Instants are INTEGER unix seconds, set by
-- the application from its injected clock.

-- events is one event page. year and slug form its address
-- /arrangementer/{year}/{slug}; both are frozen once published_at is set, so
-- links never break. year is NULL until the event has a date, and NULLs are
-- distinct in a unique index, so undated drafts (fresh copies) never collide.
CREATE TABLE events (
    id           INTEGER PRIMARY KEY,
    year         INTEGER,
    slug         TEXT    NOT NULL DEFAULT ''
        CHECK (slug NOT GLOB '*[^a-z0-9-]*'),
    title        TEXT    NOT NULL DEFAULT '',
    summary      TEXT    NOT NULL DEFAULT '',
    body_md      TEXT    NOT NULL DEFAULT '',
    start_date   TEXT    NOT NULL DEFAULT ''
        CHECK (start_date = '' OR date(start_date) IS start_date),
    start_time   TEXT    NOT NULL DEFAULT ''
        CHECK (start_time = '' OR (start_time GLOB '[0-2][0-9]:[0-5][0-9]' AND start_time < '24:00')),
    end_date     TEXT    NOT NULL DEFAULT ''
        CHECK (end_date = '' OR (date(end_date) IS end_date AND end_date >= start_date)),
    end_time     TEXT    NOT NULL DEFAULT ''
        CHECK (end_time = '' OR (end_time GLOB '[0-2][0-9]:[0-5][0-9]' AND end_time < '24:00')),
    price_text   TEXT    NOT NULL DEFAULT '',
    ticket_url   TEXT    NOT NULL DEFAULT '',
    show_in_menu INTEGER NOT NULL DEFAULT 0 CHECK (show_in_menu IN (0, 1)),
    menu_label   TEXT    NOT NULL DEFAULT '',
    status       TEXT    NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    cancelled    INTEGER NOT NULL DEFAULT 0 CHECK (cancelled IN (0, 1)),
    published_at INTEGER,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    CHECK (status = 'draft' OR (
        published_at IS NOT NULL AND year IS NOT NULL
        AND slug <> '' AND trim(title) <> '' AND start_date <> ''
    ))
);

CREATE UNIQUE INDEX events_year_slug ON events (year, slug) WHERE slug <> '';
CREATE INDEX events_slug ON events (slug, start_date);

-- event_dates are the extra days of an event that runs on several separate
-- days (a course); start_date is always the first and is not repeated here.
CREATE TABLE event_dates (
    event_id INTEGER NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    date     TEXT    NOT NULL CHECK (date(date) IS date),
    PRIMARY KEY (event_id, date)
) WITHOUT ROWID;

-- photos is one uploaded original, named by the sha256 of its content; the
-- file itself lives in originals/ and is never referenced by any other key.
CREATE TABLE photos (
    id                INTEGER PRIMARY KEY,
    sha256            TEXT    NOT NULL UNIQUE
        CHECK (length(sha256) = 64 AND sha256 NOT GLOB '*[^0-9a-f]*'),
    mime              TEXT    NOT NULL CHECK (mime <> ''),
    width             INTEGER NOT NULL CHECK (width > 0),
    height            INTEGER NOT NULL CHECK (height > 0),
    alt               TEXT    NOT NULL DEFAULT '',
    photographer_name TEXT    NOT NULL DEFAULT '',
    photographer_url  TEXT    NOT NULL DEFAULT '',
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL
);

-- photo_crops holds one rectangle per crop kind, in source pixels. A photo
-- without a crop of a kind falls back to a centred one computed by images/.
CREATE TABLE photo_crops (
    photo_id INTEGER NOT NULL REFERENCES photos (id) ON DELETE CASCADE,
    kind     TEXT    NOT NULL CHECK (kind IN ('card', 'banner', 'share')),
    x        INTEGER NOT NULL CHECK (x >= 0),
    y        INTEGER NOT NULL CHECK (y >= 0),
    w        INTEGER NOT NULL CHECK (w > 0),
    h        INTEGER NOT NULL CHECK (h > 0),
    PRIMARY KEY (photo_id, kind)
) WITHOUT ROWID;

-- event_photos and gallery_photos reference photos without ON DELETE: a photo
-- that is in use cannot be deleted, so no page ever points at a missing image.
CREATE TABLE event_photos (
    event_id INTEGER NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    photo_id INTEGER NOT NULL REFERENCES photos (id),
    role     TEXT    NOT NULL CHECK (role IN ('main', 'extra')),
    position INTEGER NOT NULL CHECK (position >= 0),
    PRIMARY KEY (event_id, photo_id),
    UNIQUE (event_id, position)
) WITHOUT ROWID;

CREATE UNIQUE INDEX event_photos_one_main ON event_photos (event_id) WHERE role = 'main';
CREATE INDEX event_photos_photo ON event_photos (photo_id);

CREATE TABLE gallery_photos (
    photo_id   INTEGER PRIMARY KEY REFERENCES photos (id),
    position   INTEGER NOT NULL UNIQUE CHECK (position >= 0),
    updated_at INTEGER NOT NULL
);
