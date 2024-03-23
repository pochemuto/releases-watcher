package releaseswatcher

import (
	"context"

	"github.com/jackc/pgx/v5"
	// "github.com/sirupsen/logrus"
)

// var log logrus.Logger

// const databaseName = "music"

type DB struct {
	conn *pgx.Conn
}

type Album struct {
	Artist string `bson:"artist"`
	Album  string `bson:"album"`
}

func (a Album) IsCorrect() bool {
	return len(a.Artist) > 0 && len(a.Album) > 0
}

func (db *DB) Disconnect() error {
	return db.conn.Close(context.Background())
}

func (db *DB) Insert(ctx context.Context, album Album) error {
	_, err := db.conn.Exec(ctx,
		"INSERT INTO album (artist, name) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		album.Artist, album.Album)
	return err
}

func NewDB(connection string) (DB, error) {
	conn, err := pgx.Connect(context.Background(), connection)
	if err != nil {
		return DB{}, nil
	}

	return DB{conn: conn}, nil
}
