-- name: GetLocalAlbums :many
SELECT *
FROM album;
-- name: GetActualAlbums :many
SELECT *
FROM actual_album;
-- name: GetLocalArtists :many
SELECT DISTINCT artist
FROM album;
-- name: GetAll :many
SELECT value,
	id
FROM cache
WHERE entity = $1;
-- name: DeleteAllLocalAlbums :exec
DELETE FROM album;
-- name: DeleteAllActualAlbums :exec
DELETE FROM actual_album;
-- name: InsertLocalAlbum :exec
INSERT INTO album (artist, name)
VALUES ($1, $2) ON CONFLICT DO NOTHING;
-- name: InsertActualAlbum :exec
INSERT INTO actual_album (id, artist, name, year, kind, version_id)
VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING;
-- name: GetCache :one
SELECT value
FROM cache
WHERE entity = $1
	AND id = $2;
-- name: InsertCache :exec
INSERT INTO cache (entity, id, value)
VALUES ($1, $2, $3);
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
-- name: CreateActualAlbumPartition :exec
SELECT create_actual_album_partition(@version::int);