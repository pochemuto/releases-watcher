//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package releaseswatcher

import "github.com/google/wire"

func InitializeApp(
	connection string,
	discogsToken string,
	root string,
) (DB, Watcher) {
	wire.Build(NewDB, NewLibrary, NewWatcher)
	return DB{}, Watcher{}
}
