//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package releaseswatcher

import (
	"context"
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/google/wire"
)

type Application struct {
	DB      DB
	Watcher Watcher
	Differ  Differ
	Sheets  GoogleSheets
}

type Config struct {
	WatcherConfig `envDefault:""`
	Db            DbConfig           `envPrefix:"DB_" envDefault:""`
	Diff          DifferConfig       `envPrefix:"DIFF_" envDefault:""`
	Discogs       DiscogsConfig      `envPrefix:"DISCOGS_" envDefault:""`
	MusicBrainz   MusicBrainzConfig  `envPrefix:"MUSIC_BRAINZ_" envDefault:""`
	GoogleSheets  GoogleSheetsConfig `envPrefix:"GOOGLE_SHEETS_" envDefault:""`
}

func NewApplication(
	db DB,
	watcher Watcher,
	differ Differ,
	sheets GoogleSheets,
) Application {
	return Application{
		DB:      db,
		Watcher: watcher,
		Differ:  differ,
		Sheets:  sheets,
	}
}

func InitializeApplication(ctx context.Context) (Application, error) {
	var config Config
	err := env.ParseWithOptions(&config, env.Options{RequiredIfNoDef: true, UseFieldNameByDefault: true})
	if err != nil {
		return Application{}, fmt.Errorf("parsing env variables error: %w", err)
	}

	app, err := initializeApp(ctx, config)
	if err != nil {
		return Application{}, fmt.Errorf("app initialization error: %w", err)
	}

	return app, nil
}

func initializeApp(
	ctx context.Context,
	config Config,
) (Application, error) {
	wire.Build(
		NewDB,
		NewMusicBrainzLibrary,
		wire.Bind(new(Library), new(MusicBrainzLibrary)),
		NewWatcher,
		NewApplication,
		NewCache,
		NewPgxPool,
		NewDiffer,
		NewGoogleSheets,
		wire.FieldsOf(new(Config), "Db", "Diff", "MusicBrainz", "GoogleSheets", "WatcherConfig"),
	)
	return Application{}, nil
}
