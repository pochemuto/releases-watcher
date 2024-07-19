//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package releaseswatcher

import "github.com/google/wire"

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

func InitializeApp(
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
