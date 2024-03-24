package releaseswatcher

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	// "github.com/sirupsen/logrus"
)

type DB struct {
	conn *pgxpool.Pool
}

type Album struct {
	Artist string `bson:"artist"`
	Album  string `bson:"album"`
}

func NewDB(connection string) (DB, error) {
	conn, err := pgxpool.New(context.Background(), connection)
	if err != nil {
		return DB{}, err
	}

	err = conn.Ping(context.Background())
	if err != nil {
		return DB{}, err
	}
	return DB{conn: conn}, nil
}

func (a Album) IsCorrect() bool {
	return len(a.Artist) > 0 && len(a.Album) > 0
}

func (db DB) Disconnect() {
	db.conn.Close()
}

func (db DB) InsertLocalAlbum(ctx context.Context, album Album) error {
	_, err := db.conn.Exec(ctx,
		"INSERT INTO album (artist, name) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		album.Artist, album.Album)
	return err
}

func (db DB) GetRelease(ctx context.Context, releaseID int) ([]byte, error) {
	row := db.conn.QueryRow(ctx,
		"SELECT response FROM discogs.release WHERE id = $1",
		releaseID,
	)
	var result []byte
	err := row.Scan(&result)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}

func (db DB) SaveRelease(ctx context.Context, releaseID int, response []byte) error {
	_, err := db.conn.Exec(ctx,
		"INSERT INTO discogs.release (id, response) VALUES ($1, $2)",
		releaseID, response,
	)
	return err
}
