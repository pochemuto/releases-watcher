package releaseswatcher

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/pochemuto/releases-watcher/sqlc"
)

type Differ struct {
	db         DB
	cutoffYear int
}

func NewDiffer(db DB) Differ {
	cutoffYear, err := strconv.Atoi(os.Getenv("ALBUMS_CUTOFF_YEAR"))
	if err != nil {
		log.Warnf("Error parsing ALBUMS_CUTOFF_YEAR: %v", err)
	}
	return Differ{
		db:         db,
		cutoffYear: cutoffYear,
	}
}

func toAlbum(ea sqlc.ExcludedAlbum) sqlc.LocalAlbumPublished {
	return sqlc.LocalAlbumPublished{
		Artist: ea.Artist,
		Name:   ea.Album,
	}
}

// Diff function with excluded albums and artists
func (d Differ) Diff() ([]sqlc.ActualAlbumPublished, error) {
	local, err := d.db.GetLocalAlbums(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error loading local albums: %w", err)
	}
	actual, err := d.db.GetActualAlbums(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error loading actual albums: %w", err)
	}
	log.Infof("Loaded %d local albums and %d actual albums", len(local), len(actual))

	excludedArtists, err := d.db.queries.GetExcludedArtists(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error when loading excluded artists: %w", err)
	}
	log.Infof("Excluded artists %d", len(excludedArtists))
	excludedAlbums, err := d.db.queries.GetExcludedAlbums(context.Background())
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

	// Create a set of normalized excluded artists for faster lookup.
	excludedArtistMap := make(map[string]bool)
	for _, artist := range excludedArtists {
		normalized := normalizeString(artist)
		excludedArtistMap[normalized] = true
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
			log.Tracef("Album exists locally, skipping: %s", normalizedAlbum)
			continue
		}
		if _, albumOk := excludedAlbumMap[normalizedAlbum]; albumOk {
			log.Tracef("Album is excluded, skipping: %s", normalizedAlbum)
			continue
		}
		if _, artistOk := excludedArtistMap[normalizedArtist]; artistOk {
			log.Tracef("Artist is excluded, skipping: %s", normalizedArtist)
			continue
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
