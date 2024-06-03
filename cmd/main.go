package main

import (
	"context"
	"flag"
	"os"
	"sort"

	"github.com/joho/godotenv"
	releaseswatcher "github.com/pochemuto/releases-watcher/internal"
	"github.com/pochemuto/releases-watcher/sqlc"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func main() {
	updateLocal := flag.Bool("update-local", false, "Update local library")
	updateActual := flag.Bool("update-actual", false, "Update actual library")
	flag.Parse()

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	connectionString := os.Getenv("PGCONNECTION")
	if connectionString == "" {
		log.Panicf("Provide a connection string PGCONNECTION")
	}
	db, err := releaseswatcher.NewDB(connectionString)
	if err != nil {
		log.Panicf("Error connecting to db %v", err)
	}
	defer db.Disconnect()

	lib, err := releaseswatcher.NewLibrary(os.Getenv("DISCOGS_TOKEN"), db)
	if err != nil {
		log.Panicf("Library creation error %v", err)
	}
	watcher, err := releaseswatcher.NewWatcher("/Users/pochemuto/Music", db, lib)
	if err != nil {
		log.Panicf("Watcher creation error: %v", err)
	}

	if *updateLocal {
		err = watcher.UpdateLocalLibrary()
		if err != nil {
			log.Panicf("Update local library error: %v", err)
		}
	}
	if *updateActual {
		err = watcher.UpdateActualLibrary()
		if err != nil {
			log.Panicf("Update actual library error: %v", err)
		}
	}

	local, err := db.GetLocalAlbums(context.Background())
	if err != nil {
		log.Panicf("Error loading local albums: %v", err)
	}
	actual, err := db.GetActualAlbums(context.Background())
	if err != nil {
		log.Panicf("Error loading actual albums: %v", err)
	}

	excludedAlbums := []sqlc.Album{
		{Artist: "Jamie XX", Name: "In Colours"},
	}

	excludedArtists := []string{
		"Oasis",
		"Skillet",
		"Red",
		"Juno Reactor",
		"GMS",
		"Klaxons",
		"Matt & Kim",
		"Sum 41",
		"Three Days Grace",
		"Michael Jackson",
		"Maxim",
		"Papa Roach",
		"Evanescence",
		"Fireflight",
		"Venetian Snares",
		"The Naked and Famous",
	}

	newAlbums := releaseswatcher.Diff(local, actual, excludedAlbums, excludedArtists)
	albumCount := 0
	// Sort newAlbums by Year in descending order
	sort.Slice(newAlbums, func(i, j int) bool {
		return *newAlbums[i].Year < *newAlbums[j].Year
	})
	for _, newAlbum := range newAlbums {
		if *newAlbum.Kind == "album" {
			albumCount++
			log.Infof("New album: [%v] %s - %s (%s)",
				*newAlbum.Year,
				*newAlbum.Artist, *newAlbum.Name, *newAlbum.Kind)
		}
	}

	log.Infof("Found %d new albums", albumCount)
	log.Info("Done")
}
