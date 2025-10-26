//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package releaseswatcher

import (
	"context"
	"fmt"
	"os"

	"github.com/google/wire"
	"github.com/joho/godotenv"
)

type Application struct {
	DB             DB
	Watcher        Watcher
	Differ         Differ
	ArtistSettings ArtistSettingsSheet
}

func NewApplication(
	db DB,
	watcher Watcher,
	differ Differ,
	artistSettings ArtistSettingsSheet,
) Application {
	return Application{
		DB:             db,
		Watcher:        watcher,
		Differ:         differ,
		ArtistSettings: artistSettings,
	}
}

func InitializeApplication(ctx context.Context) (Application, error) {
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

	spreadsheetID := SpreadsheetID(os.Getenv("GOOGLE_SHEETS_SPREADSHEET_ID"))

	app, err := initializeApp(ctx, connectionString, musicbrainzToken, root, spreadsheetID)
	if err != nil {
		return Application{}, fmt.Errorf("app initialization error: %w", err)
	}

	return app, nil
}

func initializeApp(
	ctx context.Context,
	connection ConnectionString,
	token MusicBrainzToken,
	root RootPath,
	spreadsheetID SpreadsheetID,
) (Application, error) {
	wire.Build(NewDB,
		NewMusicBrainzLibrary,
		wire.Bind(new(Library), new(MusicBrainzLibrary)),
		NewWatcher,
		NewApplication,
		NewCache,
		NewPgxPool,
		NewDiffer,
		NewArtistSettingsSheet,
	)
	return Application{}, nil
}
