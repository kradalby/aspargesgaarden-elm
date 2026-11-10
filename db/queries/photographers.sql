-- name: CreatePhotographer :one
INSERT INTO photographers (name, name_key, url) VALUES (?, ?, ?)
RETURNING id;

-- name: EnsurePhotographer :one
-- A known name returns the existing row's id with its link left alone; the
-- no-op DO UPDATE is there because SQLite cannot RETURN from DO NOTHING.
INSERT INTO photographers (name, name_key, url) VALUES (?, ?, ?)
ON CONFLICT (name_key) DO UPDATE SET name_key = excluded.name_key
RETURNING id;

-- name: GetPhotographer :one
SELECT * FROM photographers WHERE id = ?;

-- name: ListPhotographers :many
SELECT * FROM photographers ORDER BY name_key;

-- name: UpdatePhotographer :execrows
UPDATE photographers SET name = ?, name_key = ?, url = ? WHERE id = ?;

-- name: DeletePhotographer :execrows
DELETE FROM photographers WHERE id = ?;

-- name: GetPhotographerByNameKey :one
SELECT * FROM photographers WHERE name_key = ?;
