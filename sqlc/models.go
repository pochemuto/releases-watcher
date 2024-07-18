// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0

package sqlc

import (
	"github.com/jackc/pgx/v5/pgtype"
)

type ActualAlbum struct {
	ID     int64
	Artist *string
	Name   *string
	Year   *int32
	Kind   *string
}

type Album struct {
	Artist string
	Name   string
}

type Cache struct {
	Entity string
	ID     string
	Value  []byte
	Ts     pgtype.Timestamp
}

type ExcludedAlbum struct {
	Artist string
	Album  string
}

type ExcludedArtist struct {
	Artist string
}
