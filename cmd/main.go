package main

import (
	"context"
	"flag"
	"os/signal"
	"sort"
	"syscall"

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

func run(ctx context.Context) {
	updateLocal := flag.Bool("update-local", false, "Update local library")
	updateActual := flag.Bool("update-actual", false, "Update actual library")
	diff := flag.Bool("diff", false, "Print diff")
	updateSettings := flag.Bool("update-settings", false, "Write local artists to Google Sheets settings")
	flag.Parse()

	app, err := releaseswatcher.InitializeApplication(ctx)
	if err != nil {
		log.Fatal(err)
	}
	watcher, differ := app.Watcher, app.Differ
	if *updateLocal {
		err = watcher.UpdateLocalLibrary(ctx)
		if err != nil {
			log.Fatalf("update local library error: %v", err)
		}
	}
	if *updateActual {
		err = watcher.UpdateActualLibrary(ctx)
		if err != nil {
			log.Fatalf("update actual library error: %v", err)
		}
	}

	if *updateSettings {
		artists, err := app.DB.GetLocalArtists(ctx)
		if err != nil {
			log.Fatalf("load local artists error: %v", err)
		}
		err = app.Sheets.UpdateArtistsInSettings(ctx, artists)
		if err != nil {
			log.Fatalf("update settings in sheet error: %v", err)
		}
	}

	if *diff {
		newAlbums, err := differ.Diff(ctx)
		if err != nil {
			log.Fatalf("error making diff: %v", err)
		}
		albumCount := 0
		// Sort newAlbums by Year in descending order
		sort.Slice(newAlbums, func(i, j int) bool {
			if *newAlbums[i].Artist != *newAlbums[j].Artist {
				return *newAlbums[i].Artist < *newAlbums[j].Artist
			}
			return *newAlbums[i].Year < *newAlbums[j].Year
		})
		for _, newAlbum := range newAlbums {
			if *newAlbum.Kind == "Album" {
				albumCount++
				log.Infof("New album: [%v] %s - %s (%s)  https://musicbrainz.org/release/%v",
					*newAlbum.Year,
					*newAlbum.Artist, *newAlbum.Name, *newAlbum.Kind, newAlbum.ID)
			}
		}
		app.Sheets.UpdateReleases(ctx, newAlbums)
		log.Infof("Found %d new albums", albumCount)
	}
	log.Info("Done")
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	run(ctx)
}
