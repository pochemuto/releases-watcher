package releaseswatcher

import (
	"regexp"
	"strings"

	"github.com/pochemuto/releases-watcher/sqlc"
)

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
	regex := `[^a-zA-Z0-9★]`
	return strings.ToLower(regexp.MustCompile(regex).ReplaceAllString(s, ""))
}

func Normalize(a sqlc.Album) sqlc.Album {
	return sqlc.Album{
		Artist: normalizeString(a.Artist),
		Name:   normalizeString(a.Name),
	}
}

// Diff function with excluded albums and artists
func Diff(local []sqlc.Album, actual []sqlc.ActualAlbum, excludedAlbums []sqlc.Album, excludedArtists []string) []sqlc.ActualAlbum {
	// Create a map of normalized local albums for faster lookup.
	localMap := make(map[sqlc.Album]bool)
	for _, local := range local {
		normalized := Normalize(local)
		localMap[normalized] = true
	}

	// Create a set of normalized excluded albums for faster lookup.
	excludedAlbumMap := make(map[sqlc.Album]bool)
	for _, album := range excludedAlbums {
		normalized := Normalize(album)
		excludedAlbumMap[normalized] = true
	}

	// Create a set of normalized excluded artists for faster lookup.
	excludedArtistMap := make(map[string]bool)
	for _, artist := range excludedArtists {
		normalized := normalizeString(artist)
		excludedArtistMap[normalized] = true
	}

	// Iterate over actual albums and check if they exist in the map or are excluded.
	result := make([]sqlc.ActualAlbum, 0)
	for _, actual := range actual {
		if actual.Year != nil && *actual.Year < 2010 {
			continue
		}

		normalizedAlbum := Normalize(sqlc.Album{Artist: *actual.Artist, Name: *actual.Name})
		normalizedArtist := normalizeString(*actual.Artist)

		if _, localOk := localMap[normalizedAlbum]; localOk {
			continue
		}
		if _, albumOk := excludedAlbumMap[normalizedAlbum]; albumOk {
			continue
		}
		if _, artistOk := excludedArtistMap[normalizedArtist]; artistOk {
			continue
		}

		result = append(result, actual)
	}

	return result
}
