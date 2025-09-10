-- name: GetLocalAlbums :many
SELECT *
FROM local_album_published;
-- name: GetActualAlbums :many
SELECT *
FROM actual_album_published;
-- name: GetLocalArtists :many
SELECT DISTINCT artist
FROM local_album_published;
-- name: GetAll :many
SELECT value,
	id
FROM cache
WHERE entity = $1;
-- name: InsertLocalAlbum :exec
INSERT INTO local_album (artist, name, version_id)
VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;
-- name: InsertActualAlbum :exec
INSERT INTO actual_album (id, artist, name, year, kind, version_id)
VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING;
-- name: GetCache :one
SELECT value
FROM cache
WHERE entity = $1
	AND id = $2
	AND ts >= $3;
-- name: InsertCache :exec
INSERT INTO cache (entity, id, value)
VALUES ($1, $2, $3) ON CONFLICT (entity, id) DO
UPDATE
SET value = EXCLUDED.value,
	ts = CURRENT_TIMESTAMP;
-- name: GetExcludedArtists :many
SELECT artist
FROM excluded_artist;
-- name: GetExcludedAlbums :many
SELECT artist,
	album
FROM excluded_album;
-- name: CreateActualVersion :one
INSERT INTO actual_version (published)
VALUES (FALSE)
RETURNING version_id,
	created_at,
	published;
-- name: CreateLocalVersion :one
INSERT INTO local_version (published)
VALUES (FALSE)
RETURNING version_id,
	created_at,
	published;
-- name: CreateActualAlbumPartition :exec
SELECT create_actual_album_partition(@version::int);
-- name: CreateLocalAlbumPartition :exec
SELECT create_local_album_partition(@version::int);
-- name: PublishActualVersion :exec
UPDATE actual_version
SET published = TRUE
WHERE version_id = @version::int;
-- name: PublishLocalVersion :exec
UPDATE local_version
SET published = TRUE
WHERE version_id = @version::int;