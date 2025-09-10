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

func init() {
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		PadLevelText:    true,
	})
}

type RootPath string

type Library interface {
	GetActualAlbumsForArtists(ctx context.Context, artists []string, out chan<- sqlc.ActualAlbum)
}

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

	excludedArtists, err := w.db.GetExcludedArtists(context.Background())
	if err != nil {
		return fmt.Errorf("error loading excluded artists: %w", err)
	}

	// Filter out excluded artists
	filteredArtists := []string{}
	for _, artist := range artists {
		if !contains(excludedArtists, artist) {
			filteredArtists = append(filteredArtists, artist)
		}
	}

	version, err := w.db.CreateActualVersion(context.Background())
	if err != nil {
		return fmt.Errorf("error creating new version: %w", err)
	}
	actualAlbums := make(chan sqlc.ActualAlbum, 100)
	go w.lib.GetActualAlbumsForArtists(context.Background(), filteredArtists, actualAlbums)
	count := 0
	for actualAlbum := range actualAlbums {
		actualAlbum.VersionID = version.VersionID
		err := w.db.InsertActualAlbum(context.Background(), actualAlbum)
		if err != nil {
			return fmt.Errorf("error inserting actual album: %w", err)
		}
		count++
		if count%100 == 0 {
			log.Infof("Inserted %d actual albums", count)
		}
	}
	err = w.db.PublishActualVersion(context.Background(), version)
	if err != nil {
		return fmt.Errorf("error publishing actual version: %w", err)
	}
	log.Infof("Inserted total %d actual albums in version %d", count, version.VersionID)
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

	for range ReadWorkers {
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

	version, err := w.db.CreateLocalVersion(context.Background())
	if err != nil {
		return fmt.Errorf("error creating new version: %w", err)
	}
	albums := make(map[sqlc.LocalAlbum]bool)
	for tag := range tags {
		album := sqlc.LocalAlbum{
			Artist: tag.Artist(),
			Name:   tag.Album(),
		}
		if _, present := albums[album]; !present {
			albums[album] = true
			if !IsCorrect(album) {
				log.Warnf("Incorrect tag %v", tag)
			}
			album.VersionID = version.VersionID
			err := w.db.InsertLocalAlbum(context.Background(), album)
			if err != nil {
				log.Errorf("Failed to write to db: %v", err)
			}
			log.Tracef("Read %d/%d %s - %s", processedCount.Load(), filesnamesCount.Load(),
				album.Artist, album.Name)
		}
	}

	err = w.db.PublishLocalVersion(context.Background(), version)
	if err != nil {
		return fmt.Errorf("error publishing local version: %w", err)
	}
	log.Infof("Inserted total %d local albums in version %d", len(albums), version.VersionID)
	return nil
}

func contains(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}
