package releaseswatcher

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pochemuto/releases-watcher/sqlc"
	// "github.com/sirupsen/logrus"
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

func IsCorrect(a sqlc.Album) bool {
	return len(a.Artist) > 0 && len(a.Name) > 0
}

func (db DB) Disconnect() {
	db.conn.Close()
}

func (db DB) StartUpdateLocalAlbums(ctx context.Context) (pgx.Tx, error) {
	tx, err := db.conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	err = db.queries.WithTx(tx).DeleteAllLocalAlbums(ctx)
	if err != nil {
		tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}

func (db DB) StartUpdateActualAlbums(ctx context.Context) (pgx.Tx, error) {
	tx, err := db.conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	err = db.queries.WithTx(tx).DeleteAllActualAlbums(ctx)
	if err != nil {
		tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}

func (db DB) InsertLocalAlbum(ctx context.Context, tx pgx.Tx, album sqlc.Album) error {
	return db.queries.WithTx(tx).InsertLocalAlbum(ctx, sqlc.InsertLocalAlbumParams(album))
}

func (db DB) InsertActualAlbum(ctx context.Context, tx pgx.Tx, album sqlc.ActualAlbum) error {
	return db.queries.WithTx(tx).InsertActualAlbum(ctx, sqlc.InsertActualAlbumParams(album))
}

func (db DB) GetLocalAlbums(ctx context.Context) ([]sqlc.Album, error) {
	return db.queries.GetLocalAlbums(ctx)
}

func (db DB) GetActualAlbums(ctx context.Context) ([]sqlc.ActualAlbum, error) {
	return db.queries.GetActualAlbums(ctx)
}

func (db DB) GetLocalArtists(ctx context.Context) ([]string, error) {
	return db.queries.GetLocalArtists(ctx)
}
