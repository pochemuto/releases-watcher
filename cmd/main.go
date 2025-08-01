package main

import (
	"flag"
	"os"
	"sort"

	"github.com/pochemuto/releases-watcher/internal/releaseswatcher"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func init() {
	log.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		PadLevelText:    true,
	})
	log.SetReportCaller(true)
}

func main() {
	updateLocal := flag.Bool("update-local", false, "Update local library")
	updateActual := flag.Bool("update-actual", false, "Update actual library")
	diff := flag.Bool("diff", false, "Print diff")
	flag.Parse()

	app, err := releaseswatcher.InitializeApplication()
	watcher, differ := app.Watcher, app.Differ
	if err != nil {
		log.Fatal(err)
	}
	if *updateLocal {
		err = watcher.UpdateLocalLibrary()
		if err != nil {
			log.Fatalf("update local library error: %v", err)
		}
	}
	if *updateActual {
		err = watcher.UpdateActualLibrary()
		if err != nil {
			log.Fatalf("update actual library error: %v", err)
		}
	}

	if *diff {
		newAlbums, err := differ.Diff()
		if err != nil {
			log.Fatalf("error making diff: %v", err)
		}
		albumCount := 0
		// Sort newAlbums by Year in descending order
		sort.Slice(newAlbums, func(i, j int) bool {
			return *newAlbums[i].Year < *newAlbums[j].Year
		})
		for _, newAlbum := range newAlbums {
			if *newAlbum.Kind == "album" {
				albumCount++
				log.Infof("New album: [%v] %s - %s (%s)  http://discogs.com/release/%v",
					*newAlbum.Year,
					*newAlbum.Artist, *newAlbum.Name, *newAlbum.Kind, newAlbum.ID)
			}
		}

		log.Infof("Found %d new albums", albumCount)
	}
	log.Info("Done")
	os.Exit(0)
}
