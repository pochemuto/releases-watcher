package releaseswatcher

import (
	"fmt"

	"github.com/pochemuto/releases-watcher/sqlc"
)

type Kind int

const (
	KindUnknown = iota
	KindAlbum
	KindEP
	KindSingle
)

var kindName = map[Kind]string{
	KindAlbum:  "Album",
	KindEP:     "EP",
	KindSingle: "Single",
}

func (k Kind) String() string {
	return kindName[k]
}

func KindOf(published *sqlc.ActualAlbumPublished) (Kind, error) {
	for k, v := range kindName {
		if v == *published.Kind {
			return k, nil
		}
	}
	return KindUnknown, fmt.Errorf("unknown kind %v", *published.Kind)
}
