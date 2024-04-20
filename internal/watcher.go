package releaseswatcher

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/bogem/id3v2"
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
	if err != nil {
		return err
	}
	for i, artist := range artists {
		log.Infof("Fetching for %s [%d of %d]", artist, i+1, len(artists))
		releases, err := w.lib.GetReleases(artist)
		if err != nil {
			log.Errorf("Error when processing artist '%v': %v", artist, err)
			continue
		}
		for _, r := range releases {
			if IsAlbum(&r) {
				log.Infof("Album: [%d] [%s] (%d) %s",
					r.Year, "Album", r.ID, r.Title)
			}
		}
		for _, r := range releases {
			if IsEP(&r) {
				log.Infof("Album: [%d] [%s] (%d) %s",
					r.Year, "EP", r.ID, r.Title)
			}
		}
		for _, r := range releases {
			if IsSingle(&r) {
				log.Infof("Album: [%d] [%s] (%d) %s",
					r.Year, "Single", r.ID, r.Title)
			}
		}
	}
	return err
}

func (w Watcher) UpdateLocalLibrary() {
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

	albums := make(map[Album]bool)
	for tag := range tags {
		album := Album{
			Artist: tag.Artist(),
			Album:  tag.Album(),
		}
		if _, present := albums[album]; !present {
			albums[album] = true
			if !album.IsCorrect() {
				log.Warnf("Incorrect tag %v", tag)
			}
			err := w.db.InsertLocalAlbum(context.TODO(), album)
			if err != nil {
				log.Errorf("Failed to write to db: %v", err)
			}
			log.Infof("Read %d/%d %s - %s", processedCount.Load(), filesnamesCount.Load(),
				album.Artist, album.Album)
		}
	}
}
