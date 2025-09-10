package releaseswatcher

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/pochemuto/releases-watcher/sqlc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	dbResource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15",
		Env: []string{
			"POSTGRES_USER=user",
			"POSTGRES_PASSWORD=password",
			"POSTGRES_DB=testdb",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Purge(dbResource)
	})

	connString := fmt.Sprintf("postgres://user:password@localhost:%s/testdb?sslmode=disable", dbResource.GetPort("5432/tcp"))

	var pgxPool *pgxpool.Pool
	require.Eventually(t, func() bool {
		pgxPool, err = NewPgxPool(ConnectionString(connString))
		return err == nil
	}, time.Minute, time.Second)

	t.Cleanup(func() {
		pgxPool.Close()
	})

	// Apply schema
	schema, err := os.ReadFile("../../schema.sql")
	require.NoError(t, err)

	conn, err := pgxPool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), string(schema))
	require.NoError(t, err)

	return pgxPool
}

func TestNewDB(t *testing.T) {
	pool := setupTestDB(t)
	db, err := NewDB(pool)
	require.NoError(t, err)
	assert.NotNil(t, db.conn)
	assert.NotNil(t, db.queries)
}

func TestDB_GetLocalAlbums(t *testing.T) {
	pool := setupTestDB(t)
	db, err := NewDB(pool)
	require.NoError(t, err)

	ctx := context.Background()

	// Assuming the database is empty initially
	albums, err := db.GetLocalAlbums(ctx)
	require.NoError(t, err)
	assert.Empty(t, albums)
}

func TestDB_InsertAndGetLocalAlbum(t *testing.T) {
	pool := setupTestDB(t)
	db, err := NewDB(pool)
	require.NoError(t, err)

	ctx := context.Background()
	tx, err := db.StartUpdateLocalAlbums(ctx)
	require.NoError(t, err)

	album := sqlc.Album{
		Artist: "Test Artist",
		Name:   "Test Album",
	}
	err = db.InsertLocalAlbum(ctx, tx, album)
	require.NoError(t, err)
	tx.Commit(ctx)

	albums, err := db.GetLocalAlbums(ctx)
	require.NoError(t, err)
	assert.Len(t, albums, 1)
	assert.Equal(t, album.Artist, albums[0].Artist)
	assert.Equal(t, album.Name, albums[0].Name)
}

func TestDB_StartUpdateLocalAlbums(t *testing.T) {
	pool := setupTestDB(t)
	db, err := NewDB(pool)
	require.NoError(t, err)

	ctx := context.Background()
	tx, err := db.StartUpdateLocalAlbums(ctx)
	require.NoError(t, err)
	tx.Rollback(ctx) // Ensure rollback works without errors
}

func TestDB_CreateVersion(t *testing.T) {
	pool := setupTestDB(t)
	db, err := NewDB(pool)
	require.NoError(t, err)

	ctx := context.Background()

	version, err := db.CreateVersion(ctx)
	require.NoError(t, err)
	assert.NotZero(t, version.VersionID, "VersionID should not be zero")
}
