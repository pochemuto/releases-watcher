package releaseswatcher

import (
	"testing"

	"github.com/pochemuto/releases-watcher/sqlc"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    sqlc.Album
		expected sqlc.Album
	}{
		{
			name: "Normalize album with spaces and special characters",
			input: sqlc.Album{
				Artist: "The Beatles (Remastered)",
				Name:   "Abbey Road [Deluxe Edition]",
			},
			expected: sqlc.Album{
				Artist: "thebeatles",
				Name:   "abbeyroad",
			},
		},
		{
			name: "Normalize album with uppercase letters",
			input: sqlc.Album{
				Artist: "QUEEN",
				Name:   "A Night At The Opera",
			},
			expected: sqlc.Album{
				Artist: "queen",
				Name:   "anightattheopera",
			},
		},
		{
			name: "Normalize album with non-alphanumeric characters",
			input: sqlc.Album{
				Artist: "David Bowie",
				Name:   "★ (Blackstar)",
			},
			expected: sqlc.Album{
				Artist: "davidbowie",
				Name:   "★",
			},
		},
		{
			name: "Normalize empty album",
			input: sqlc.Album{
				Artist: "",
				Name:   "",
			},
			expected: sqlc.Album{
				Artist: "",
				Name:   "",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := Normalize(test.input)
			if actual != test.expected {
				t.Errorf("Normalize(%v) = %v, expected %v", test.input, actual, test.expected)
			}
		})
	}
}
