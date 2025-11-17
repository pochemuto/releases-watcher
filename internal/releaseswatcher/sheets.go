package releaseswatcher

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	settingsSheetName   = "Настройки"
	headerRange         = settingsSheetName + "!A1:B1"
	settingsDataRange   = settingsSheetName + "!A2:B"
	defaultHeaderTitle  = "Artist"
	defaultHeaderNotice = "Notification"

	releasesSheetName  = "Релизы"
	releasesRange      = releasesSheetName + "!A1:H"
	releasesClearRange = releasesSheetName + "!A:H"
)

type NotificationSetting string

const (
	NotificationAllReleases NotificationSetting = "Все релизы"
	NotificationAlbumsAndEP NotificationSetting = "Альбомы и EP"
	NotificationAlbumsOnly  NotificationSetting = "Альбомы"
	NotificationDoNotTrack  NotificationSetting = "Не отслеживать"
)

var notificationScope = map[NotificationSetting]map[Kind]bool{
	NotificationAllReleases: {
		KindAlbum:   true,
		KindEP:      true,
		KindSingle:  true,
		KindUnknown: true,
	},
	NotificationAlbumsAndEP: {
		KindAlbum:   true,
		KindEP:      true,
		KindSingle:  false,
		KindUnknown: false,
	},
	NotificationAlbumsOnly: {
		KindAlbum:   true,
		KindEP:      false,
		KindSingle:  false,
		KindUnknown: false,
	},
}

func (n NotificationSetting) IsReleaseInScope(kind Kind) bool {
	return notificationScope[n][kind]
}

func (n NotificationSetting) String() string {
	return string(n)
}

func parseNotificationSetting(raw string) (NotificationSetting, error) {
	value := strings.TrimSpace(raw)
	switch value {
	case "", string(NotificationAllReleases):
		return NotificationAllReleases, nil
	case string(NotificationAlbumsAndEP):
		return NotificationAlbumsAndEP, nil
	case string(NotificationDoNotTrack):
		return NotificationDoNotTrack, nil
	case string(NotificationAlbumsOnly):
		return NotificationAlbumsOnly, nil
	default:
		return "", fmt.Errorf("unknown notification value %q", raw)
	}
}

type ArtistSetting struct {
	ArtistName   string
	Notification NotificationSetting
}

type GoogleSheetsConfig struct {
	CredentialsFile string `envDefault:"google-credentials.json"`
	SpreadsheetID   string `envDefault:"1j-xtIRVbdzguaBoaW3l52VMmVJmOtP4lggR7O3yW9E8"`
}

type GoogleSheets struct {
	service       *sheets.Service
	spreadsheetID string
}

func NewGoogleSheets(ctx context.Context, config GoogleSheetsConfig) (GoogleSheets, error) {
	opts := []option.ClientOption{
		option.WithScopes(sheets.SpreadsheetsScope),
	}
	if config.CredentialsFile != "" {
		if _, err := os.Stat(config.CredentialsFile); err != nil {
			return GoogleSheets{}, fmt.Errorf("check credentials file: %w", err)
		}
		opts = append(opts, option.WithCredentialsFile(config.CredentialsFile))
	}

	service, err := sheets.NewService(ctx, opts...)
	if err != nil {
		return GoogleSheets{}, fmt.Errorf("failed to create sheets client: %w", err)
	}
	return GoogleSheets{
		service:       service,
		spreadsheetID: config.SpreadsheetID,
	}, nil
}

func (g *GoogleSheets) GetArtistSettings(ctx context.Context) ([]ArtistSetting, error) {
	header, settings, err := g.readSettings(ctx)
	if err != nil {
		return nil, err
	}
	_ = header // header is kept for symmetry, but not returned
	return settings, nil
}

func (g *GoogleSheets) UpdateArtistsInSettings(ctx context.Context, artists []string) error {
	header, settings, err := g.readSettings(ctx)
	if err != nil {
		return err
	}

	if len(header) == 0 || isHeaderEmpty(header) {
		newHeader := []any{defaultHeaderTitle, defaultHeaderNotice}
		valueRange := &sheets.ValueRange{
			MajorDimension: "ROWS",
			Range:          headerRange,
			Values:         [][]any{newHeader},
		}
		if _, err := g.service.Spreadsheets.Values.Update(string(g.spreadsheetID), headerRange, valueRange).
			ValueInputOption("RAW").
			Context(ctx).
			Do(); err != nil {
			return fmt.Errorf("update header: %w", err)
		}
		header = newHeader
	}

	existingNotifications := make(map[string]NotificationSetting, len(settings))
	for _, setting := range settings {
		existingNotifications[setting.ArtistName] = setting.Notification
	}

	uniqueArtists := make([]string, 0, len(artists))
	seen := make(map[string]struct{}, len(artists))
	for _, artist := range artists {
		name := strings.TrimSpace(artist)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		uniqueArtists = append(uniqueArtists, name)
	}

	sort.Strings(uniqueArtists)

	rows := make([][]any, 0, len(uniqueArtists))

	for _, artist := range uniqueArtists {
		notification, ok := existingNotifications[artist]
		if !ok {
			notification = NotificationAllReleases
		}
		rows = append(rows, []any{artist, notification.String()})
	}

	clearRequest := &sheets.ClearValuesRequest{}
	if _, err := g.service.Spreadsheets.Values.Clear(string(g.spreadsheetID), settingsDataRange, clearRequest).Context(ctx).Do(); err != nil {
		return fmt.Errorf("clear settings range: %w", err)
	}

	if len(rows) == 0 {
		return nil
	}

	valueRange := &sheets.ValueRange{
		MajorDimension: "ROWS",
		Range:          settingsDataRange,
		Values:         rows,
	}

	_, err = g.service.Spreadsheets.Values.Update(string(g.spreadsheetID), settingsDataRange, valueRange).
		ValueInputOption("RAW").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("update settings range: %w", err)
	}
	return nil
}

type releaseState struct {
	inActual bool
	inLocal  bool
}

var releaseStates = map[releaseState]string{
	releaseState{true, true}:   "В коллекции",
	releaseState{true, false}:  "Новый",
	releaseState{false, true}:  "Не найден",
	releaseState{false, false}: "Ошибка",
}

func (g *GoogleSheets) UpdateReleases(ctx context.Context, releases []MatchedAlbum) error {
	rows := make([][]any, 0, len(releases)+1)
	rows = append(rows, []any{"Артист", "Альбом", "Локальный артист", "Локальный альбом", "Тип", "Год", "Ссылка", "В коллекции"})

	sort.SliceStable(releases, func(i, j int) bool {
		a := releases[i]
		b := releases[j]

		aArtist, aYear, aName, aHasActual := releaseSortKey(a)
		bArtist, bYear, bName, bHasActual := releaseSortKey(b)

		if aArtist != bArtist {
			return aArtist < bArtist
		}

		if aHasActual && bHasActual && aYear != bYear {
			return aYear < bYear
		}

		if aName != bName {
			return aName < bName
		}

		if aHasActual != bHasActual {
			return aHasActual
		}

		return false
	})
	for _, release := range releases {
		actual := release.Actual
		artist := ""
		album := ""
		kind := ""
		year := ""
		link := ""
		if actual != nil {
			if actual.Artist != nil {
				artist = *actual.Artist
			}
			if actual.Name != nil {
				album = *actual.Name
			}
			if actual.Kind != nil {
				kind = *actual.Kind
			}
			if actual.Year != nil {
				year = fmt.Sprintf("%d", *actual.Year)
			}
			if actual.Url != nil {
				link = *actual.Url
			}
		}

		localArtist := ""
		localAlbum := ""
		if release.Local != nil {
			localArtist = release.Local.Artist
			localAlbum = release.Local.Name
		}

		inCollection := releaseStates[releaseState{inActual: release.Actual != nil, inLocal: release.Local != nil}]

		rows = append(rows, []any{artist, album, localArtist, localAlbum, kind, year, link, inCollection})
	}

	clearRequest := &sheets.ClearValuesRequest{}
	if _, err := g.service.Spreadsheets.Values.Clear(string(g.spreadsheetID), releasesClearRange, clearRequest).Context(ctx).Do(); err != nil {
		return fmt.Errorf("clear releases range: %w", err)
	}

	valueRange := &sheets.ValueRange{
		MajorDimension: "ROWS",
		Range:          releasesRange,
		Values:         rows,
	}

	if _, err := g.service.Spreadsheets.Values.Update(string(g.spreadsheetID), releasesRange, valueRange).
		ValueInputOption("RAW").
		Context(ctx).
		Do(); err != nil {
		return fmt.Errorf("update releases range: %w", err)
	}

	return nil
}

func releaseSortKey(m MatchedAlbum) (artist string, year int32, name string, hasActual bool) {
	if m.Actual != nil {
		if m.Actual.Artist != nil {
			artist = *m.Actual.Artist
		}
		if m.Actual.Year != nil {
			year = *m.Actual.Year
		}
		if m.Actual.Name != nil {
			name = *m.Actual.Name
		}
		hasActual = true
		return
	}

	if m.Local != nil {
		artist = m.Local.Artist
		name = m.Local.Name
	}

	return
}

func (g *GoogleSheets) readSettings(ctx context.Context) ([]any, []ArtistSetting, error) {
	headerResp, err := g.service.Spreadsheets.Values.Get(string(g.spreadsheetID), headerRange).
		Context(ctx).
		MajorDimension("ROWS").
		Do()
	if err != nil {
		return nil, nil, fmt.Errorf("fetch header: %w", err)
	}

	var header []any
	if headerResp != nil && len(headerResp.Values) > 0 {
		header = cloneRow(headerResp.Values[0])
	}

	resp, err := g.service.Spreadsheets.Values.Get(string(g.spreadsheetID), settingsDataRange).
		Context(ctx).
		MajorDimension("ROWS").
		Do()
	if err != nil {
		return nil, nil, fmt.Errorf("fetch settings: %w", err)
	}

	settings := make([]ArtistSetting, 0, len(resp.Values))
	for idx, row := range resp.Values {
		if len(row) == 0 {
			continue
		}
		artist := strings.TrimSpace(fmt.Sprint(row[0]))
		if artist == "" {
			continue
		}
		var notification NotificationSetting
		if len(row) > 1 {
			notification, err = parseNotificationSetting(fmt.Sprint(row[1]))
			if err != nil {
				return nil, nil, fmt.Errorf("row %d: %w", idx+2, err)
			}
		} else {
			notification = NotificationAllReleases
		}
		settings = append(settings, ArtistSetting{
			ArtistName:   artist,
			Notification: notification,
		})
	}

	return header, settings, nil
}

func cloneRow(row []any) []any {
	if row == nil {
		return nil
	}
	clone := make([]any, len(row))
	copy(clone, row)
	return clone
}

func isHeaderEmpty(row []any) bool {
	if len(row) == 0 {
		return true
	}
	for _, cell := range row {
		if strings.TrimSpace(fmt.Sprint(cell)) != "" {
			return false
		}
	}
	return true
}
