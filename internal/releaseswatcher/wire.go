//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package releaseswatcher

import "github.com/google/wire"

type Application struct {
	DB      *DB
	Watcher *Watcher
}

func NewApplication(
	db *DB,
	watcher *Watcher,
) *Application {
	return &Application{
		DB:      db,
		Watcher: watcher,
	}
}

func InitializeApp(
	connection ConnectionString,
	token DiscogsToken,
	root RootPath,
) (*Application, error) {
	wire.Build(NewDB, NewLibrary, NewWatcher, NewApplication)
	return nil, nil
}
