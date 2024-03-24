package releaseswatcher

import (
	"github.com/irlndts/go-discogs"
)

type Library struct {
	discogs discogs.Discogs
}

func NewLibrary(token string) (Library, error) {
	client, err := discogs.New(&discogs.Options{
		UserAgent: "Releases Watcher",
		Token:     token,
		URL:       "https://api.discogs.com", // optional
	})
	if err != nil {
		return Library{}, err
	}
	return Library{discogs: client}, nil
}

func (l Library) GetReleases(artist string) ([]discogs.ReleaseSource, error) {
	request := discogs.SearchRequest{Type: "artist", Q: artist, PerPage: 5}
	search, err := l.discogs.Search(request)
	if err != nil {
		return nil, err
	}
	for _, r := range search.Results {
		log.Infof("Artist: [%d] %s", r.ID, r.Title)
	}

	originalArtist := search.Results[0]
	resp, err := l.discogs.ArtistReleases(originalArtist.ID,
		&discogs.Pagination{Page: 0, PerPage: 1000, Sort: "year", SortOrder: "asc"})
	if err != nil {
		return nil, err
	}

	releases := make([]discogs.ReleaseSource, 0)
	for _, r := range resp.Releases {
		if r.Type == "master" && r.Artist == originalArtist.Title && r.Role == "Main" {
			releases = append(releases, r)
		}
	}
	return releases, nil
	// for _, r := range search.Results {
	// 	releases = append(releases, r)
	// }
}
