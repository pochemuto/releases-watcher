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
type discogs_cache struct {
	DB
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

func (db DB) Discogs() discogs_cache {
	return discogs_cache{db}
}

func (db DB) InsertLocalAlbum(ctx context.Context, album Album) error {
	_, err := db.conn.Exec(ctx,
		"INSERT INTO album (artist, name) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		album.Artist, album.Album)
	return err
}

func (db DB) GetLocalAlbums(ctx context.Context) ([]Album, error) {
	result := make([]Album, 0)
	scan, err := db.conn.Query(ctx, "SELECT artist, name FROM album")
	if err != nil {
		return nil, err
	}
	for scan.Next() {
		var album Album
		err := scan.Scan(&album.Artist, &album.Album)
		if err != nil {
			return nil, err
		}
		result = append(result, album)
	}
	return result, nil
}

func (db DB) GetLocalArtists(ctx context.Context) ([]string, error) {
	result := make([]string, 0)
	scan, err := db.conn.Query(ctx, "SELECT DISTINCT artist FROM album")
	if err != nil {
		return nil, err
	}
	for scan.Next() {
		var artist string
		err := scan.Scan(&artist)
		if err != nil {
			return nil, err
		}
		result = append(result, artist)
	}
	return result, nil
}

func (db discogs_cache) GetRelease(ctx context.Context, releaseID int) ([]byte, error) {
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

func (db discogs_cache) SaveRelease(ctx context.Context, releaseID int, response []byte) error {
	_, err := db.conn.Exec(ctx,
		"INSERT INTO discogs.release (id, response) VALUES ($1, $2)",
		releaseID, response,
	)
	return err
}
