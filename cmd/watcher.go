package main

import (
	"sync"
	"sync/atomic"

	"github.com/bogem/id3v2"
	releaseswatcher "github.com/pochemuto/releases-watcher/internal"
	"github.com/sirupsen/logrus"
)

const ReadWorkers = 10

var log = logrus.New()

func main() {
	filenames := make(chan string)
	tags := make(chan id3v2.Tag)

	var filesnamesCount atomic.Int32
	var processedCount atomic.Int32
	go releaseswatcher.Scan("/Volumes/Yandex.Disk/Music", filenames, &filesnamesCount)

	var wg sync.WaitGroup

	for i := 0; i < ReadWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filename := range filenames {
				tag, err := releaseswatcher.ReadID3(filename)
				processedCount.Add(1)
				if err != nil {
					log.Warningf("Error when parsing %s: %v", err, filename)
					continue
				}
				tags <- *tag
			}
			close(tags)
		}()
	}

	type Album struct {
		artist string
		album  string
	}
	albums := make(map[Album]bool)
	for tag := range tags {
		album := Album{
			artist: tag.Artist(),
			album:  tag.Album(),
		}
		if _, present := albums[album]; !present {
			albums[album] = true
			log.Infof("Read %d/%d %s - %s", processedCount.Load(), filesnamesCount.Load(),
				album.artist, album.album)
		}
	}
	wg.Wait()
}
