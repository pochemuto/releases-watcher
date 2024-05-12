package releaseswatcher

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/bogem/id3v2"
	"github.com/irlndts/go-discogs"
	"github.com/pochemuto/releases-watcher/sqlc"
	"github.com/sirupsen/logrus"
)

const ReadWorkers = 10

var log = logrus.New()

type Watcher struct {
	db   DB
	lib  Library
	root string
}

func NewWatcher(root string, db DB, lib Library) (Watcher, error) {
	return Watcher{root: root, db: db, lib: lib}, nil
}

func (w Watcher) UpdateActualLibrary() error {
	artists, err := w.db.GetLocalArtists(context.Background())
	log.Infof("Updating local library from Discogs for %d artists", len(artists))
	if err != nil {
		return err
	}
	tx, err := w.db.StartUpdateActualAlbums(context.Background())
	if err != nil {
		return err
	}
	defer tx.Commit(context.Background())
	for i, artist := range artists {
		log.Tracef("Fetching for %s [%d of %d]", artist, i+1, len(artists))
		releases, err := w.lib.GetReleases(artist)
		if err != nil {
			log.Errorf("Error when processing artist '%v': %v", artist, err)
			continue
		}
		for _, release := range releases {
			kind := getKind(release)
			if kind == "" {
				continue
			}
			year := int32(release.Year)
			actualAlbum := sqlc.ActualAlbum{
				ID:     int64(release.ID),
				Artist: &artist,
				Name:   &release.Title,
				Year:   &year,
				Kind:   &kind,
			}
			err := w.db.InsertActualAlbum(context.Background(), tx, actualAlbum)
			if err != nil {
				tx.Rollback(context.Background())
				return err
			}
		}
	}
	return err
}

func getKind(release discogs.Release) string {
	if IsAlbum(&release) {
		return "album"
	}
	if IsSingle(&release) {
		return "single"
	}
	if IsEP(&release) {
		return "EP"
	}
	return ""
}

func (w Watcher) UpdateLocalLibrary() error {
	log.Info("Updating local library")
	filenames := make(chan string)
	tags := make(chan id3v2.Tag)

	var filesnamesCount atomic.Int32
	var processedCount atomic.Int32
	go Scan(w.root, filenames, &filesnamesCount)

	var wg sync.WaitGroup

	for i := 0; i < ReadWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filename := range filenames {
				tag, err := ReadID3(filename)
				processedCount.Add(1)
				if err != nil {
					log.Warningf("Error when parsing %s: %v", err, filename)
					continue
				}
				tags <- *tag
			}
		}()
	}
	go func() {
		wg.Wait()
		close(tags)
	}()

	albums := make(map[sqlc.Album]bool)

	tx, err := w.db.StartUpdateLocalAlbums(context.Background())
	if err != nil {
		return err
	}
	defer tx.Commit(context.Background())
	for tag := range tags {
		album := sqlc.Album{
			Artist: tag.Artist(),
			Name:   tag.Album(),
		}
		if _, present := albums[album]; !present {
			albums[album] = true
			if !IsCorrect(album) {
				log.Warnf("Incorrect tag %v", tag)
			}
			err := w.db.InsertLocalAlbum(context.TODO(), tx, album)
			if err != nil {
				log.Errorf("Failed to write to db: %v", err)
			}
			log.Tracef("Read %d/%d %s - %s", processedCount.Load(), filesnamesCount.Load(),
				album.Artist, album.Name)
		}
	}

	return nil
}
