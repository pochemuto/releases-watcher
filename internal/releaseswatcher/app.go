package releaseswatcher

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/joho/godotenv"
)

func App(updateLocal *bool, updateActual *bool) error {
	err := godotenv.Load()
	if err != nil {
		return fmt.Errorf("error loading .env file: %w", err)
	}

	connectionString := ConnectionString(os.Getenv("PGCONNECTION"))
	if connectionString == "" {
		return fmt.Errorf("provide a connection string PGCONNECTION")
	}
	discogsToken := DiscogsToken(os.Getenv("DISCOGS_TOKEN"))
	if discogsToken == "" {
		return fmt.Errorf("provide a DISCOGS_TOKEN")
	}
	root := RootPath(os.Getenv("ROOT"))
	if root == "" {
		return fmt.Errorf("provide a ROOT")
	}

	app, err := InitializeApp(connectionString, discogsToken, root)
	if err != nil {
		return fmt.Errorf("app initialization error: %w", err)
	}
	db, watcher, differ := app.DB, app.Watcher, app.Differ
	defer db.Disconnect()

	if *updateLocal {
		err = watcher.UpdateLocalLibrary()
		if err != nil {
			return fmt.Errorf("update local library error: %w", err)
		}
	}
	if *updateActual {
		err = watcher.UpdateActualLibrary()
		if err != nil {
			return fmt.Errorf("update actual library error: %w", err)
		}
	}

	local, err := db.GetLocalAlbums(context.Background())
	if err != nil {
		return fmt.Errorf("error loading local albums: %w", err)
	}
	actual, err := db.GetActualAlbums(context.Background())
	if err != nil {
		return fmt.Errorf("error loading actual albums: %w", err)
	}

	newAlbums, err := differ.Diff(local, actual)
	if err != nil {
		return fmt.Errorf("error making diff: %w", err)
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
	log.Info("Done")
	return nil
}
