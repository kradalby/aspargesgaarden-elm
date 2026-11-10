-- name: GetSetting :one
SELECT value FROM settings WHERE name = ?;

-- name: InsertSettingIfMissing :exec
-- A value once made is never replaced: two starts racing keep the first.
INSERT INTO settings (name, value) VALUES (?, ?)
ON CONFLICT (name) DO NOTHING;
