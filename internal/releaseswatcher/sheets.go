package releaseswatcher

import (
	"context"
	"fmt"
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
)

type SpreadsheetID string

const DefaultSpreadsheetID SpreadsheetID = "1j-xtIRVbdzguaBoaW3l52VMmVJmOtP4lggR7O3yW9E8"

type NotificationSetting string

const (
	NotificationAllReleases NotificationSetting = "Все релизы"
	NotificationAlbumsAndEP NotificationSetting = "Альбомы и EP"
	NotificationDoNotTrack  NotificationSetting = "Не отслеживать"
	NotificationAlbumsOnly  NotificationSetting = "Альбомы"
)

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

type ArtistSettingsSheet interface {
	GetArtistSettings(ctx context.Context) ([]ArtistSetting, error)
	UpdateArtistsInSettings(ctx context.Context, artists []string) error
}

type GoogleSheetsArtistSettings struct {
	service       *sheets.Service
	spreadsheetID SpreadsheetID
}

func NewArtistSettingsSheet(ctx context.Context, spreadsheetID SpreadsheetID) (ArtistSettingsSheet, error) {
	service, err := sheets.NewService(ctx, option.WithScopes(sheets.SpreadsheetsScope))
	if err != nil {
		return nil, fmt.Errorf("failed to create sheets client: %w", err)
	}
	if spreadsheetID == "" {
		spreadsheetID = DefaultSpreadsheetID
	}
	return &GoogleSheetsArtistSettings{
		service:       service,
		spreadsheetID: spreadsheetID,
	}, nil
}

func (g *GoogleSheetsArtistSettings) GetArtistSettings(ctx context.Context) ([]ArtistSetting, error) {
	header, settings, err := g.readSettings(ctx)
	if err != nil {
		return nil, err
	}
	_ = header // header is kept for symmetry, but not returned
	return settings, nil
}

func (g *GoogleSheetsArtistSettings) UpdateArtistsInSettings(ctx context.Context, artists []string) error {
	header, settings, err := g.readSettings(ctx)
	if err != nil {
		return err
	}

	if len(header) == 0 || isHeaderEmpty(header) {
		newHeader := []interface{}{defaultHeaderTitle, defaultHeaderNotice}
		valueRange := &sheets.ValueRange{
			MajorDimension: "ROWS",
			Range:          headerRange,
			Values:         [][]interface{}{newHeader},
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

	rows := make([][]interface{}, 0, len(uniqueArtists))

	for _, artist := range uniqueArtists {
		notification, ok := existingNotifications[artist]
		if !ok {
			notification = NotificationAllReleases
		}
		rows = append(rows, []interface{}{artist, notification.String()})
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

func (g *GoogleSheetsArtistSettings) readSettings(ctx context.Context) ([]interface{}, []ArtistSetting, error) {
	headerResp, err := g.service.Spreadsheets.Values.Get(string(g.spreadsheetID), headerRange).
		Context(ctx).
		MajorDimension("ROWS").
		Do()
	if err != nil {
		return nil, nil, fmt.Errorf("fetch header: %w", err)
	}

	var header []interface{}
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

func cloneRow(row []interface{}) []interface{} {
	if row == nil {
		return nil
	}
	clone := make([]interface{}, len(row))
	copy(clone, row)
	return clone
}

func isHeaderEmpty(row []interface{}) bool {
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
