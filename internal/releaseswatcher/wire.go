//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package releaseswatcher

import (
	"fmt"
	"os"

	"github.com/google/wire"
	"github.com/joho/godotenv"
)

type Application struct {
	DB      DB
	Watcher Watcher
	Differ  Differ
}

func NewApplication(
	db DB,
	watcher Watcher,
	differ Differ,
) Application {
	return Application{
		DB:      db,
		Watcher: watcher,
		Differ:  differ,
	}
}

func InitializeApplication() (Application, error) {
	err := godotenv.Load()
	if err != nil {
		return Application{}, fmt.Errorf("error loading .env file: %w", err)
	}

	connectionString := ConnectionString(os.Getenv("PGCONNECTION"))
	if connectionString == "" {
		return Application{}, fmt.Errorf("provide a connection string PGCONNECTION")
	}
	musicbrainzToken := MusicBrainzToken(os.Getenv("MUSICBRAINZ_TOKEN"))
	if musicbrainzToken == "" {
		return Application{}, fmt.Errorf("provide a MUSICBRAINZ_TOKEN")
	}
	root := RootPath(os.Getenv("ROOT"))
	if root == "" {
		return Application{}, fmt.Errorf("provide a ROOT")
	}

	app, err := initializeApp(connectionString, musicbrainzToken, root)
	if err != nil {
		return Application{}, fmt.Errorf("app initialization error: %w", err)
	}

	return app, nil
}

func initializeApp(
	connection ConnectionString,
	token MusicBrainzToken,
	root RootPath,
) (Application, error) {
	wire.Build(NewDB,
		NewMusicBrainzLibrary,
		wire.Bind(new(Library), new(MusicBrainzLibrary)),
		NewWatcher,
		NewApplication,
		NewCache,
		NewPgxPool,
		NewDiffer,
	)
	return Application{}, nil
}
