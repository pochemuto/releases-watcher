package main

import (
	"flag"
	"os"

	"github.com/pochemuto/releases-watcher/internal/releaseswatcher"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func main() {
	updateLocal := flag.Bool("update-local", false, "Update local library")
	updateActual := flag.Bool("update-actual", false, "Update actual library")
	flag.Parse()

	err := releaseswatcher.App(updateLocal, updateActual)
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}
