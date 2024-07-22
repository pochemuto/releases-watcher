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
	discogsToken := DiscogsToken(os.Getenv("DISCOGS_TOKEN"))
	if discogsToken == "" {
		return Application{}, fmt.Errorf("provide a DISCOGS_TOKEN")
	}
	root := RootPath(os.Getenv("ROOT"))
	if root == "" {
		return Application{}, fmt.Errorf("provide a ROOT")
	}

	app, err := initializeApp(connectionString, discogsToken, root)
	if err != nil {
		return Application{}, fmt.Errorf("app initialization error: %w", err)
	}

	return app, nil
}

func initializeApp(
	connection ConnectionString,
	token DiscogsToken,
	root RootPath,
) (Application, error) {
	wire.Build(NewDB,
		NewLibrary,
		NewWatcher,
		NewApplication,
		NewCache,
		NewPgxPool,
		NewDiffer,
	)
	return Application{}, nil
}
