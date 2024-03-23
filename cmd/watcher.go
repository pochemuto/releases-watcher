package main

import (
	"context"
	"os"
	"sync"
	"sync/atomic"

	"github.com/bogem/id3v2"
	"github.com/joho/godotenv"
	releaseswatcher "github.com/pochemuto/releases-watcher/internal"
	"github.com/sirupsen/logrus"
)

const ReadWorkers = 10

var log = logrus.New()

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

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
		}()
	}
	go func() {
		wg.Wait()
		close(tags)
	}()

	connectionString := os.Getenv("PGCONNECTION")
	if connectionString == "" {
		log.Panicf("Provide a connection string PGCONNECTION")
	}
	db, err := releaseswatcher.NewDB(connectionString)
	if err != nil {
		log.Panicf("Error connecting to db %v", err)
	}
	defer db.Disconnect()

	albums := make(map[releaseswatcher.Album]bool)
	for tag := range tags {
		album := releaseswatcher.Album{
			Artist: tag.Artist(),
			Album:  tag.Album(),
		}
		if _, present := albums[album]; !present {
			albums[album] = true
			if !album.IsCorrect() {
				log.Warnf("Incorrect tag %v", tag)
			}
			err := db.Insert(context.TODO(), album)
			if err != nil {
				log.Errorf("Failed to write to db: %v", err)
			}
			log.Infof("Read %d/%d %s - %s", processedCount.Load(), filesnamesCount.Load(),
				album.Artist, album.Album)
		}
	}
}
