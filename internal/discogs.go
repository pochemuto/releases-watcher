package releaseswatcher

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/irlndts/go-discogs"
	"golang.org/x/crypto/openpgp/errors"
	"golang.org/x/time/rate"
)

type Library struct {
	db       DB
	discogs  discogs.Discogs
	limiter  *rate.Limiter
	releases map[string]*discogs.Release
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

func (l *Library) api() discogs.Discogs {
	l.limiter.Wait(context.Background())
	return l.discogs
}

func (l *Library) getRelease(releaseID int) (*discogs.Release, error) {
	id := strconv.Itoa(releaseID)
	freshness := 10 * 24 * time.Hour
	if l.releases == nil {
		var err error
		l.releases, err = GetAll[discogs.Release](&l.db, context.TODO(), "discogs_release", freshness)
		if err != nil {
			return nil, err
		}
		log.Infof("Loaded %d releases from cache", len(l.releases))
	}
	if release, ok := l.releases[id]; ok {
		log.Tracef("Loaded release %d from cache", releaseID)
		return release, nil
	}
	return GetCached(&l.db, context.TODO(), "discogs_release", id,
		freshness, func() (*discogs.Release, error) {
			return l.api().Release(releaseID)
		})
}

func IsAlbum(release *discogs.Release) bool {
	for _, format := range release.Formats {
		for _, desc := range format.Descriptions {
			if strings.ToLower(desc) == "album" {
				return true
			}
		}
	}
	return false
}

func IsEP(release *discogs.Release) bool {
	for _, format := range release.Formats {
		for _, desc := range format.Descriptions {
			if strings.ToLower(desc) == "ep" {
				return true
			}
		}
	}
	return false
}

func IsSingle(release *discogs.Release) bool {
	for _, format := range release.Formats {
		for _, desc := range format.Descriptions {
			if strings.ToLower(desc) == "single" {
				return true
			}
		}
	}
	return false
}

func (l *Library) getArtistID(artist string) (int, error) {
	search, err := GetCached(&l.db, context.TODO(), "discogs_artist_search", artist, 10*24*time.Hour, func() (*discogs.Search, error) {
		request := discogs.SearchRequest{Type: "artist", Q: artist, PerPage: 300}
		return l.api().Search(request)
	})
	if err != nil {
		return 0, err
	}
	if len(search.Results) == 0 {
		return 0, errors.InvalidArgumentError("Artist '" + artist + "' not found")
	}
	return search.Results[0].ID, nil
}

func (l *Library) getArtistReleases(artistID int, page int) (*discogs.ArtistReleases, error) {
	id := fmt.Sprintf("%d_%d", artistID, page)
	return GetCached(&l.db, context.TODO(), "discord_artist_releases",
		id, 10*24*time.Hour, func() (*discogs.ArtistReleases, error) {
			return l.api().ArtistReleases(artistID,
				&discogs.Pagination{Page: page, PerPage: 500, Sort: "year", SortOrder: "asc"})
		})
}

func (l *Library) GetReleases(artist string) ([]discogs.Release, error) {
	artistID, err := l.getArtistID(artist)
	if err != nil {
		return nil, err
	}
	releases := make([]discogs.Release, 0)
	page := 0
	for {
		if err != nil {
			return nil, err
		}

		resp, err := l.getArtistReleases(artistID, page)
		if err != nil {
			return nil, err
		}
		for i, r := range resp.Releases {
			if r.Type == "master" && r.Role == "Main" {
				log.Tracef("--- Fetched %d of %d [page %d/%d]", i+1, len(resp.Releases), page+1, resp.Pagination.Pages)
				release, err := l.getRelease(r.MainRelease)
				if err != nil {
					return nil, err
				}
				if IsAlbum(release) || IsEP(release) || IsSingle(release) {
					releases = append(releases, *release)
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
