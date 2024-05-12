package main

import (
	"os"

	"github.com/joho/godotenv"
	releaseswatcher "github.com/pochemuto/releases-watcher/internal"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func main() {
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
	err = watcher.UpdateLocalLibrary()
	if err != nil {
		log.Panicf("Update local library error: %v", err)
	}
	err = watcher.UpdateActualLibrary()
	if err != nil {
		log.Panicf("Update actual library error: %v", err)
	}
	log.Info("Done")
}
