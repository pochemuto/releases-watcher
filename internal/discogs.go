package releaseswatcher

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/irlndts/go-discogs"
	"golang.org/x/crypto/openpgp/errors"
	"golang.org/x/time/rate"
)

type Library struct {
	db      DB
	discogs discogs.Discogs
	limiter *rate.Limiter
}

func NewLibrary(token string, db DB) (Library, error) {
	if token == "" {
		return Library{}, errors.InvalidArgumentError("Token is empty")
	}
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
		limiter: rate.NewLimiter(50*rate.Every(time.Minute), 1),
	}, nil
}

func (l Library) api() discogs.Discogs {
	l.limiter.Wait(context.Background())
	return l.discogs
}

func (l Library) getReleaseCached(releaseID int) (discogs.Release, error) {
	cached, err := l.db.Discogs().GetRelease(context.Background(), releaseID)
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
	l.db.Discogs().SaveRelease(context.Background(), releaseID, cached)
	return *resp, nil
}

func IsAlbum(release discogs.Release) bool {
	for _, format := range release.Formats {
		for _, desc := range format.Descriptions {
			if strings.ToLower(desc) == "album" {
				return true
			}
		}
	}
	return false
}

func IsEP(release discogs.Release) bool {
	for _, format := range release.Formats {
		for _, desc := range format.Descriptions {
			if strings.ToLower(desc) == "ep" {
				return true
			}
		}
	}
	return false
}

func IsSingle(release discogs.Release) bool {
	for _, format := range release.Formats {
		for _, desc := range format.Descriptions {
			if strings.ToLower(desc) == "single" {
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

	if len(search.Results) == 0 {
		log.Warnf("Artist '%s' not found", artist)
		return make([]discogs.Release, 0), nil
	}
	originalArtist := search.Results[0]
	releases := make([]discogs.Release, 0)
	page := 0
	for {
		resp, err := l.api().ArtistReleases(originalArtist.ID,
			&discogs.Pagination{Page: page, PerPage: 500, Sort: "year", SortOrder: "asc"})
		if err != nil {
			return nil, err
		}

		for i, r := range resp.Releases {
			if r.Type == "master" && r.Role == "Main" {
				log.Infof("--- Fetched %d of %d", i+1, len(resp.Releases))
				release, err := l.getReleaseCached(r.MainRelease)
				if err != nil {
					return nil, err
				}
				if IsAlbum(release) || IsEP(release) || IsSingle(release) {
					releases = append(releases, release)
				}
			}
		}

		page = page + 1
		if page == resp.Pagination.Pages {
			break
		}

	}
	return releases, nil
}
