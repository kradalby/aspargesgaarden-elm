-- name: CreatePhoto :one
-- The same upload twice is one photo: on a known sha256 the existing row's id
-- is returned and its fields are left alone. The no-op DO UPDATE is there
-- because SQLite cannot RETURN from a bare DO NOTHING.
INSERT INTO photos (
    sha256, mime, width, height, alt, photographer_id, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (sha256) DO UPDATE SET sha256 = excluded.sha256
RETURNING id;

-- name: GetPhoto :one
SELECT * FROM photos WHERE id = ?;

-- name: GetPhotoIDBySha256 :one
-- images/ names photos by the sha256 of their original.
SELECT id FROM photos WHERE sha256 = ?;

-- name: ListPhotos :many
SELECT * FROM photos ORDER BY id;

-- name: UpdatePhotoMeta :execrows
UPDATE photos
SET alt = ?, photographer_id = ?, updated_at = ?
WHERE id = ?;

-- name: TouchPhoto :exec
UPDATE photos SET updated_at = ? WHERE id = ?;

-- name: TouchPhotographerPhotos :exec
-- A photographer's new name or link changes the pages that credit them.
UPDATE photos SET updated_at = ? WHERE photographer_id = ?;

-- name: DeletePhoto :execrows
DELETE FROM photos WHERE id = ?;

-- name: UpsertPhotoCrop :exec
INSERT INTO photo_crops (photo_id, kind, x, y, w, h) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (photo_id, kind) DO UPDATE SET
    x = excluded.x, y = excluded.y, w = excluded.w, h = excluded.h;

-- name: ListPhotoCrops :many
SELECT * FROM photo_crops WHERE photo_id = ? ORDER BY kind;

-- name: ListAllPhotoCrops :many
SELECT * FROM photo_crops ORDER BY photo_id, kind;

-- name: ListGalleryPhotoIDs :many
SELECT photo_id FROM gallery_photos ORDER BY position;

-- name: DeleteGallery :exec
DELETE FROM gallery_photos;

-- name: InsertGalleryPhoto :exec
INSERT INTO gallery_photos (photo_id, position, updated_at) VALUES (?, ?, ?);

-- name: GalleryUpdatedAt :one
-- The gallery page's lastmod: the newest change to its order or its photos.
SELECT CAST(coalesce(max(max(gallery_photos.updated_at), max(photos.updated_at)), 0) AS INTEGER)
FROM gallery_photos
JOIN photos ON photos.id = gallery_photos.photo_id;

-- name: ListPhotoEvents :many
-- The events that show a photo, which keep it from being deleted.
SELECT events.id, events.title FROM events
JOIN event_photos ON event_photos.event_id = events.id
WHERE event_photos.photo_id = ?
ORDER BY events.id;

-- name: DeleteGalleryPhoto :exec
DELETE FROM gallery_photos WHERE photo_id = ?;

-- name: DeleteUnusedPhotos :many
-- Photos no event or gallery uses, not uploaded, edited or taken out of use
-- (ReleasePhotos) since the cutoff. The check and the delete are one
-- statement, so a save that links one of them either lands first and keeps
-- it, or fails on the foreign key.
DELETE FROM photos
WHERE photos.updated_at < ?
    AND NOT EXISTS (SELECT 1 FROM event_photos WHERE event_photos.photo_id = photos.id)
    AND NOT EXISTS (SELECT 1 FROM gallery_photos WHERE gallery_photos.photo_id = photos.id)
RETURNING sha256;

-- name: ReleasePhotos :exec
-- Of photos just taken out of an event or the gallery, those nothing else
-- uses start their unused grace period now (DeleteUnusedPhotos).
UPDATE photos SET updated_at = ?
WHERE id IN (sqlc.slice('ids'))
    AND NOT EXISTS (SELECT 1 FROM event_photos WHERE event_photos.photo_id = photos.id)
    AND NOT EXISTS (SELECT 1 FROM gallery_photos WHERE gallery_photos.photo_id = photos.id);
