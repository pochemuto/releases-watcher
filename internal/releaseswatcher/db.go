package releaseswatcher

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pochemuto/releases-watcher/sqlc"
)

type DB struct {
	conn    *pgxpool.Pool
	queries *sqlc.Queries
}

type ConnectionString string

func NewPgxPool(connection ConnectionString) (*pgxpool.Pool, error) {
	conn, err := pgxpool.New(context.Background(), string(connection))
	if err != nil {
		return nil, err
	}
	err = conn.Ping(context.Background())
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func NewDB(conn *pgxpool.Pool) (DB, error) {
	return DB{
		conn:    conn,
		queries: sqlc.New(conn),
	}, nil
}

func IsCorrect(a sqlc.LocalAlbum) bool {
	return len(a.Artist) > 0 && len(a.Name) > 0
}

func (db DB) Disconnect() {
	db.conn.Close()
}

func (db DB) InsertLocalAlbum(ctx context.Context, album sqlc.LocalAlbum) error {
	return db.queries.InsertLocalAlbum(ctx, sqlc.InsertLocalAlbumParams(album))
}

func (db DB) InsertActualAlbum(ctx context.Context, album sqlc.ActualAlbum) error {
	return db.queries.InsertActualAlbum(ctx, sqlc.InsertActualAlbumParams(album))
}

func (db DB) GetLocalAlbums(ctx context.Context) ([]sqlc.LocalAlbumPublished, error) {
	return db.queries.GetLocalAlbums(ctx)
}

func (db DB) GetActualAlbums(ctx context.Context) ([]sqlc.ActualAlbumPublished, error) {
	return db.queries.GetActualAlbums(ctx)
}

func (db DB) GetLocalArtists(ctx context.Context) ([]string, error) {
	return db.queries.GetLocalArtists(ctx)
}

func (db DB) GetExcludedArtists(ctx context.Context) ([]string, error) {
	return db.queries.GetExcludedArtists(ctx)
}

func (db DB) CreateActualVersion(ctx context.Context) (sqlc.ActualVersion, error) {
	version, err := db.queries.CreateActualVersion(ctx)
	if err != nil {
		return sqlc.ActualVersion{}, err
	}
	err = db.queries.CreateActualAlbumPartition(ctx, version.VersionID)
	if err != nil {
		return sqlc.ActualVersion{}, fmt.Errorf("error creating actual album partition: %w", err)
	}
	return version, nil
}

func (db DB) CreateLocalVersion(ctx context.Context) (sqlc.LocalVersion, error) {
	version, err := db.queries.CreateLocalVersion(ctx)
	if err != nil {
		return sqlc.LocalVersion{}, err
	}
	err = db.queries.CreateLocalAlbumPartition(ctx, version.VersionID)
	if err != nil {
		return sqlc.LocalVersion{}, fmt.Errorf("error creating local album partition: %w", err)
	}
	return version, nil
}

func (db DB) PublishActualVersion(ctx context.Context, version sqlc.ActualVersion) error {
	return db.queries.PublishActualVersion(ctx, version.VersionID)
}

func (db DB) PublishLocalVersion(ctx context.Context, version sqlc.LocalVersion) error {
	return db.queries.PublishLocalVersion(ctx, version.VersionID)
}
