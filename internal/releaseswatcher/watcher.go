package releaseswatcher

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/dhowden/tag"
	"github.com/pochemuto/releases-watcher/sqlc"
	"github.com/sirupsen/logrus"
)

const ReadWorkers = 10

var log = logrus.New()

type RootPath string

type Watcher struct {
	db   DB
	lib  Library
	root RootPath
}

func NewWatcher(root RootPath, db DB, lib Library) (Watcher, error) {
	return Watcher{root: root, db: db, lib: lib}, nil
}

func (w Watcher) UpdateActualLibrary() error {
	artists, err := w.db.GetLocalArtists(context.Background())
	log.Infof("Updating local library from Discogs for %d artists", len(artists))
	if err != nil {
		return fmt.Errorf("error loading local artists: %w", err)
	}
	actualAlbums, err := w.lib.GetActualAlbumsForArtists(artists)
	if err != nil {
		return fmt.Errorf("error getting actual albums: %w", err)
	}
	tx, err := w.db.StartUpdateActualAlbums(context.Background())
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Commit(context.Background())
	for _, actualAlbum := range actualAlbums {
		err := w.db.InsertActualAlbum(context.Background(), tx, actualAlbum)
		if err != nil {
			tx.Rollback(context.Background())
			return fmt.Errorf("error inserting actual album: %w", err)
		}
	}
	return nil
}

func (w Watcher) UpdateLocalLibrary() error {
	log.Info("Updating local library")
	filenames := make(chan string)
	tags := make(chan tag.Metadata)

	var filesnamesCount atomic.Int32
	var processedCount atomic.Int32
	go Scan(string(w.root), filenames, &filesnamesCount)

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
				tags <- tag
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
			err := w.db.InsertLocalAlbum(context.Background(), tx, album)
			if err != nil {
				log.Errorf("Failed to write to db: %v", err)
			}
			log.Tracef("Read %d/%d %s - %s", processedCount.Load(), filesnamesCount.Load(),
				album.Artist, album.Name)
		}
	}

	return nil
}
