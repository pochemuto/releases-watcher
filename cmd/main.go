package main

import (
	"context"
	"errors"
	"flag"
	"io/fs"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
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

	err := godotenv.Load()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatalf("error loading .env file: %v", err)
	}

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
		matched, err := differ.Matched(ctx)
		if err != nil {
			log.Fatalf("error making diff: %v", err)
		}
		releaseCount := 0
		for _, release := range matched {
			releaseCount++
			if release.Local == nil {
				actual := release.Actual
				log.Infof("New album: [%v] %s - %s (%s)  https://musicbrainz.org/release/%v",
					actual.Year,
					*actual.Artist, *actual.Name, *actual.Kind, actual.ID)
			}
		}
		if err = app.Sheets.UpdateReleases(ctx, matched); err != nil {
			log.Errorf("Error updating releases: %v", err)
		}
		log.Infof("Found %d new albums", releaseCount)
	}
	log.Info("Done")
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	run(ctx)
}
