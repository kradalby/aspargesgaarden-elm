-- name: CreateEvent :one
INSERT INTO events (
    year, slug, title, summary, body_md,
    start_date, start_time, end_date, end_time,
    price_text, ticket_url, show_in_menu, menu_label,
    status, cancelled, published_at, created_at, updated_at
) VALUES (
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?, ?
)
RETURNING id;

-- name: UpdateEvent :exec
UPDATE events SET
    year = ?, slug = ?, title = ?, summary = ?, body_md = ?,
    start_date = ?, start_time = ?, end_date = ?, end_time = ?,
    price_text = ?, ticket_url = ?, show_in_menu = ?, menu_label = ?,
    status = ?, cancelled = ?, published_at = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteEvent :execrows
DELETE FROM events WHERE id = ?;

-- name: GetEvent :one
SELECT * FROM events WHERE id = ?;

-- name: ListEvents :many
-- Every event, drafts included, for the admin overview.
SELECT * FROM events ORDER BY start_date DESC, id DESC;

-- name: ListPublishedEvents :many
-- Upcoming vs past depends on the clock and on the extra dates, so the split
-- and its ordering are site's job; this only fixes a stable base order.
SELECT * FROM events WHERE status = 'published' ORDER BY start_date, id;

-- name: GetPublishedEventByYearSlug :one
SELECT * FROM events
WHERE status = 'published' AND year = ? AND slug = ?;

-- name: GetNewestPublishedEventBySlug :one
-- Behind evergreen addresses such as /julemarked.
SELECT * FROM events
WHERE status = 'published' AND slug = ?
ORDER BY start_date DESC, id DESC
LIMIT 1;

-- name: ListPublishedMenuEvents :many
SELECT * FROM events
WHERE status = 'published' AND show_in_menu = 1
ORDER BY start_date, id;

-- name: ListAllEventDates :many
SELECT event_id, date FROM event_dates ORDER BY event_id, date;

-- name: DeleteEventDates :exec
DELETE FROM event_dates WHERE event_id = ?;

-- name: InsertEventDate :exec
INSERT INTO event_dates (event_id, date) VALUES (?, ?);

-- name: ListAllEventPhotos :many
SELECT event_id, photo_id, role, position FROM event_photos
ORDER BY event_id, position;

-- name: DeleteEventPhotos :exec
DELETE FROM event_photos WHERE event_id = ?;

-- name: InsertEventPhoto :exec
INSERT INTO event_photos (event_id, photo_id, role, position) VALUES (?, ?, ?, ?);
