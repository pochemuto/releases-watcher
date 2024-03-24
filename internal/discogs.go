package releaseswatcher

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/irlndts/go-discogs"
	"golang.org/x/time/rate"
)

type Library struct {
	db      DB
	discogs discogs.Discogs
	limiter *rate.Limiter
}

func NewLibrary(token string, db DB) (Library, error) {
	client, err := discogs.New(&discogs.Options{
		UserAgent: "Releases Watcher",
		Token:     token,
		URL:       "https://api.discogs.com", // optional
	})
	if err != nil {
		return Library{}, err
	}
	return Library{
		db:      db,
		discogs: client,
		limiter: rate.NewLimiter(rate.Every(time.Minute), 60),
	}, nil
}

func (l Library) api() discogs.Discogs {
	l.limiter.Wait(context.Background())
	return l.discogs
}

func (l Library) getReleaseCached(releaseID int) (discogs.Release, error) {
	cached, err := l.db.GetRelease(context.Background(), releaseID)
	if err != nil {
		return discogs.Release{}, err
	}
	if cached != nil {
		var release discogs.Release
		json.Unmarshal(cached, &release)
		return release, nil
	}
	resp, err := l.api().Release(releaseID)
	if err != nil {
		return discogs.Release{}, err
	}
	cached, err = json.Marshal(resp)
	if err != nil {
		return discogs.Release{}, err
	}
	l.db.SaveRelease(context.Background(), releaseID, cached)
	return *resp, nil
}

func isAlbum(release discogs.Release) bool {
	for _, format := range release.Formats {
		for _, desc := range format.Descriptions {
			if strings.ToLower(desc) == "album" {
				return true
			}
		}
	}
	return false
}

func (l Library) GetReleases(artist string) ([]discogs.Release, error) {
	request := discogs.SearchRequest{Type: "artist", Q: artist, PerPage: 5}
	search, err := l.api().Search(request)
	if err != nil {
		return nil, err
	}
	for _, r := range search.Results {
		log.Infof("Artist: [%d] %s", r.ID, r.Title)
	}

	originalArtist := search.Results[0]
	resp, err := l.api().ArtistReleases(originalArtist.ID,
		&discogs.Pagination{Page: 0, PerPage: 1000, Sort: "year", SortOrder: "asc"})
	if err != nil {
		return nil, err
	}

	releases := make([]discogs.Release, 0)
	for _, r := range resp.Releases {
		if r.Type == "master" && r.Artist == originalArtist.Title && r.Role == "Main" {
			release, err := l.getReleaseCached(r.MainRelease)
			if err != nil {
				return nil, err
			}
			if isAlbum(release) {
				releases = append(releases, release)
			}
		}
	}
	return releases, nil
}
