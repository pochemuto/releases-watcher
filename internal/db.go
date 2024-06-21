package releaseswatcher

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pochemuto/releases-watcher/sqlc"
	// "github.com/sirupsen/logrus"
)

type DB struct {
	conn    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewDB(connection string) (*DB, error) {
	conn, err := pgxpool.New(context.Background(), connection)
	if err != nil {
		return nil, err
	}

	err = conn.Ping(context.Background())
	if err != nil {
		return nil, err
	}
	return &DB{
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

func GetAllCacheEntities[T any](db *DB, ctx context.Context,
	entity string, freshness time.Duration) (map[string]*T, error) {
	rows, err := db.GetAllCacheEntities(ctx, entity, freshness)
	if err != nil {
		return nil, err
	}
	result := make(map[string]*T)
	for id, data := range rows {
		value := new(T)
		json.Unmarshal(data, value)
		result[id] = value
	}
	return result, nil
}

func GetCached[T any](db *DB, ctx context.Context,
	entity string, id string, freshness time.Duration, fetcher func() (*T, error)) (*T, error) {
	byte_fetcher := func() ([]byte, error) {
		data, err := fetcher()
		if err != nil {
			return nil, err
		}
		return json.Marshal(data)
	}

	data, err := db.GetEntity(ctx, entity, id, freshness, byte_fetcher)
	if err != nil {
		return nil, err
	}
	result := new(T)
	err = json.Unmarshal(data, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (db DB) GetAllCacheEntities(ctx context.Context,
	entity string, freshness time.Duration) (map[string][]byte, error) {
	// use iterator
	// https://github.com/sqlc-dev/sqlc/issues/720
	rows, err := db.queries.GetAll(ctx, entity)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for _, row := range rows {
		result[row.ID] = row.Value
	}
	return result, nil
}

func (db DB) GetEntity(ctx context.Context,
	entity string, id string, freshness time.Duration, fetcher func() ([]byte, error)) ([]byte, error) {
	result, err := db.queries.GetCache(ctx, sqlc.GetCacheParams{
		Entity: entity,
		ID:     id,
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}

		result, err = fetcher()
		if err != nil {
			return nil, err
		}

		err = db.queries.InsertCache(ctx, sqlc.InsertCacheParams{
			Entity: entity,
			ID:     id,
			Value:  result,
		})
		if err != nil {
			return nil, err
		}

		return result, nil
	}
	return result, nil
}
