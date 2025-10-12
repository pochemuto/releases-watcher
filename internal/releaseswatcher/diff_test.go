package releaseswatcher

import (
	"testing"

	"github.com/pochemuto/releases-watcher/sqlc"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    sqlc.LocalAlbumPublished
		expected sqlc.LocalAlbumPublished
	}{
		{
			name: "Normalize album with spaces and special characters",
			input: sqlc.LocalAlbumPublished{
				Artist: "The Beatles (Remastered)",
				Name:   "Abbey Road [Deluxe Edition]",
			},
			expected: sqlc.LocalAlbumPublished{
				Artist: "thebeatles",
				Name:   "abbeyroad",
			},
		},
		{
			name: "Normalize album with uppercase letters",
			input: sqlc.LocalAlbumPublished{
				Artist: "QUEEN",
				Name:   "A Night At The Opera",
			},
			expected: sqlc.LocalAlbumPublished{
				Artist: "queen",
				Name:   "anightattheopera",
			},
		},
		{
			name: "Normalize album with non-alphanumeric characters",
			input: sqlc.LocalAlbumPublished{
				Artist: "David Bowie",
				Name:   "★ (Blackstar)",
			},
			expected: sqlc.LocalAlbumPublished{
				Artist: "davidbowie",
				Name:   "★",
			},
		},
		{
			name: "Normalize empty album",
			input: sqlc.LocalAlbumPublished{
				Artist: "",
				Name:   "",
			},
			expected: sqlc.LocalAlbumPublished{
				Artist: "",
				Name:   "",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := normalize(test.input)
			if actual != test.expected {
				t.Errorf("Normalize(%v) = %v, expected %v", test.input, actual, test.expected)
			}
		})
	}
}
