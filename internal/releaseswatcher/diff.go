package releaseswatcher

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/pochemuto/releases-watcher/sqlc"
)

type Differ struct {
	db         DB
	sheets     GoogleSheets
	cutoffYear uint
}

type DifferConfig struct {
	CutoffYear uint
}

func NewDiffer(db DB, config DifferConfig, sheets GoogleSheets) Differ {
	return Differ{
		db:         db,
		sheets:     sheets,
		cutoffYear: config.CutoffYear,
	}
}

func toAlbum(ea sqlc.ExcludedAlbum) sqlc.LocalAlbumPublished {
	return sqlc.LocalAlbumPublished{
		Artist: ea.Artist,
		Name:   ea.Album,
	}
}

// Diff function with excluded albums and artists
func (d Differ) Diff(ctx context.Context) ([]sqlc.ActualAlbumPublished, error) {
	local, err := d.db.GetLocalAlbums(ctx)
	if err != nil {
		return nil, fmt.Errorf("error loading local albums: %w", err)
	}
	actual, err := d.db.GetActualAlbums(ctx)
	if err != nil {
		return nil, fmt.Errorf("error loading actual albums: %w", err)
	}
	log.Infof("Loaded %d local albums and %d actual albums", len(local), len(actual))

	excludedArtists, err := d.db.queries.GetExcludedArtists(ctx)
	if err != nil {
		return nil, fmt.Errorf("error when loading excluded artists: %w", err)
	}
	log.Infof("Excluded artists %d", len(excludedArtists))
	excludedAlbums, err := d.db.queries.GetExcludedAlbums(ctx)
	if err != nil {
		return nil, fmt.Errorf("error when loading excluded albums: %w", err)
	}
	log.Infof("Excluded albums %d", len(excludedAlbums))
	// Create a map of normalized local albums for faster lookup.
	localMap := make(map[sqlc.LocalAlbumPublished]bool)
	for _, local := range local {
		normalized := normalize(local)
		localMap[normalized] = true
	}

	// Create a set of normalized excluded albums for faster lookup.
	excludedAlbumMap := make(map[sqlc.LocalAlbumPublished]bool)
	for _, album := range excludedAlbums {
		normalized := normalize(toAlbum(album))
		excludedAlbumMap[normalized] = true
	}

	settings, err := d.sheets.GetArtistSettings(ctx)
	artistSettings := make(map[string]NotificationSetting)
	for _, setting := range settings {
		normalized := normalizeString(setting.ArtistName)
		artistSettings[normalized] = setting.Notification
	}

	log.Infof("Filtering albums released since %d", d.cutoffYear)
	// Iterate over actual albums and check if they exist in the map or are excluded.
	result := make([]sqlc.ActualAlbumPublished, 0)
	for _, actual := range actual {
		if actual.Year != nil && *actual.Year < int32(d.cutoffYear) {
			continue
		}

		normalizedAlbum := normalize(sqlc.LocalAlbumPublished{Artist: *actual.Artist, Name: *actual.Name})
		normalizedArtist := normalizeString(*actual.Artist)

		if _, localOk := localMap[normalizedAlbum]; localOk {
			log.Tracef("Album exists locally, skipping: %v", normalizedAlbum)
			continue
		}
		if _, albumOk := excludedAlbumMap[normalizedAlbum]; albumOk {
			log.Tracef("Album is excluded, skipping: %v", normalizedAlbum)
			continue
		}
		if setting, artistOk := artistSettings[normalizedArtist]; artistOk {
			if setting == NotificationDoNotTrack {
				log.Tracef("Artist is excluded, skipping: %v", normalizedArtist)
				continue
			}
			kind, err := KindOf(&actual)
			if err != nil {
				log.Errorf("error getting album kind: %v", err)
				continue
			}
			if !setting.IsReleaseInScope(kind) {
				log.Tracef("Release type %v is not in scope of %v", kind, setting)
				continue
			}
		}
		result = append(result, actual)
	}

	return result, nil
}

func normalize(a sqlc.LocalAlbumPublished) sqlc.LocalAlbumPublished {
	return sqlc.LocalAlbumPublished{
		Artist:    normalizeString(a.Artist),
		Name:      normalizeString(a.Name),
		VersionID: 0,
	}
}

// Remove text in braces and brackets
func removeTextInBraces(text string) string {
	regex := `[\(\[][^\)\]]*[\)\]]`
	return regexp.MustCompile(regex).ReplaceAllString(text, "")
}

// Normalize the string by converting to lowercase and removing non-alphanumeric characters except for the star
func normalizeString(s string) string {
	// First, remove text in braces
	s = removeTextInBraces(s)
	// Now, remove unwanted characters except for letters, digits, and the star symbol
	regex := `[^\p{L}a-zA-Z0-9★]`
	return strings.ToLower(regexp.MustCompile(regex).ReplaceAllString(s, ""))
}
