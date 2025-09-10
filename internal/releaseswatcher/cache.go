package releaseswatcher

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pochemuto/releases-watcher/sqlc"
)

type Cache struct {
	conn    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewCache(conn *pgxpool.Pool) Cache {
	return Cache{
		conn:    conn,
		queries: sqlc.New(conn),
	}
}

func GetAllCacheEntities[T any](c Cache, ctx context.Context,
	entity string, freshness time.Duration) (map[string]*T, error) {
	rows, err := c.getAllCacheEntities(ctx, entity, freshness)
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

func GetCached[T any](c Cache, ctx context.Context,
	entity string, id string, freshness time.Duration, fetcher func() (*T, error)) (*T, error) {
	byte_fetcher := func() ([]byte, error) {
		data, err := fetcher()
		if err != nil {
			return nil, err
		}
		return json.Marshal(data)
	}

	data, err := c.getEntity(ctx, entity, id, freshness, byte_fetcher)
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

func (c Cache) getAllCacheEntities(ctx context.Context,
	entity string, freshness time.Duration) (map[string][]byte, error) {
	// use iterator
	// https://github.com/sqlc-dev/sqlc/issues/720
	rows, err := c.queries.GetAll(ctx, entity)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for _, row := range rows {
		result[row.ID] = row.Value
	}
	return result, nil
}

func (c Cache) getEntity(ctx context.Context,
	entity string, id string, freshness time.Duration, fetcher func() ([]byte, error)) ([]byte, error) {
	result, err := c.queries.GetCache(ctx, sqlc.GetCacheParams{
		Entity: entity,
		ID:     id,
		Ts:     pgtype.Timestamp{Time: time.Now().Add(-freshness), Valid: true},
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}

		result, err = fetcher()
		if err != nil {
			return nil, err
		}

		err = c.queries.InsertCache(ctx, sqlc.InsertCacheParams{
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
