package releaseswatcher

import (
	"context"
	"fmt"
	"strings"
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

type WatcherConfig struct {
	RootPath     string
	ExcludedPath string `envDefault:""`
}

type Library interface {
	GetActualAlbumsForArtists(ctx context.Context, artists []string, out chan<- sqlc.ActualAlbum)
	Name() string
}

type Watcher struct {
	db           DB
	lib          Library
	root         string
	excludedPath string
}

func NewWatcher(config WatcherConfig, db DB, lib Library) (Watcher, error) {
	return Watcher{root: config.RootPath, excludedPath: config.ExcludedPath, db: db, lib: lib}, nil
}

func (w Watcher) UpdateActualLibrary(ctx context.Context) error {
	artists, err := w.db.GetLocalArtists(ctx)
	log.Infof("Updating local library from %s for %d artists", w.lib.Name(), len(artists))
	if err != nil {
		return fmt.Errorf("error loading local artists: %w", err)
	}

	excludedArtists, err := w.db.GetExcludedArtists(ctx)
	if err != nil {
		return fmt.Errorf("error loading excluded artists: %w", err)
	}

	// Filter out excluded artists
	var filteredArtists []string
	for _, artist := range artists {
		if !contains(excludedArtists, artist) {
			filteredArtists = append(filteredArtists, artist)
		}
	}

	version, err := w.db.CreateActualVersion(ctx)
	if err != nil {
		return fmt.Errorf("error creating new version: %w", err)
	}
	actualAlbums := make(chan sqlc.ActualAlbum, 100)
	go w.lib.GetActualAlbumsForArtists(ctx, filteredArtists, actualAlbums)
	count := 0
	for actualAlbum := range actualAlbums {
		select {
		case <-ctx.Done():
			log.Infof("Context is done, stopping inserting actual albums")
			return nil
		default:
		}
		actualAlbum.VersionID = version.VersionID
		err := w.db.InsertActualAlbum(ctx, actualAlbum)
		if err != nil {
			return fmt.Errorf("error inserting actual album: %w", err)
		}
		count++
		if count%100 == 0 {
			log.Infof("Inserted %d actual albums", count)
		}
	}
	err = w.db.PublishActualVersion(ctx, version)
	if err != nil {
		return fmt.Errorf("error publishing actual version: %w", err)
	}
	log.Infof("Inserted total %d actual albums in version %d", count, version.VersionID)
	return nil
}

func (w Watcher) UpdateLocalLibrary(ctx context.Context) error {
	log.Info("Updating local library")
	filenames := make(chan string)
	tags := make(chan tag.Metadata)

	var filenameCount atomic.Int32
	var processedCount atomic.Int32
	go func() {
		err := Scan(ctx, string(w.root), w.excludedPath,
			filenames, &filenameCount)
		if err != nil {
			log.Errorf("Error scanning directory: %v", err)
		}
	}()

	var wg sync.WaitGroup

	for range ReadWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filename := range filenames {
				select {
				case <-ctx.Done():
					log.Infof("Context is done, stopping worker")
					return
				default:
				}
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

	version, err := w.db.CreateLocalVersion(ctx)
	if err != nil {
		return fmt.Errorf("error creating new version: %w", err)
	}
	albums := make(map[sqlc.LocalAlbum]bool)
	for tag := range tags {
		select {
		case <-ctx.Done():
			log.Infof("Context is done, stopping inserting local albums")
			return nil
		default:
		}
		albumKey := sqlc.LocalAlbum{
			Artist: strings.TrimSpace(tag.Artist()),
			Name:   strings.TrimSpace(tag.Album()),
		}
		if !IsCorrect(albumKey) {
			log.Warnf("Incorrect tag %v", tag)
			continue
		}
		if _, present := albums[albumKey]; present {
			continue
		}
		albums[albumKey] = true
		album := albumKey
		album.VersionID = version.VersionID
		err := w.db.InsertLocalAlbum(ctx, album)
		if err != nil {
			log.Errorf("Failed to write to db: %v", err)
		}
		log.Tracef("Read %d/%d %s - %s", processedCount.Load(), filenameCount.Load(),
			album.Artist, album.Name)
	}

	err = w.db.PublishLocalVersion(ctx, version)
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
