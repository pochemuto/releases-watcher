package releaseswatcher

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/irlndts/go-discogs"
	"github.com/pochemuto/releases-watcher/sqlc"
	"golang.org/x/crypto/openpgp/errors"
	"golang.org/x/time/rate"
)

type cached struct {
	releases map[string]*discogs.Release
}

type DiscogsLibrary struct {
	db      DB
	cache   Cache
	discogs discogs.Discogs
	limiter *rate.Limiter
	cached  *cached
}

type DiscogsToken string

func NewDiscogsLibrary(token DiscogsToken, db DB, cache Cache) (DiscogsLibrary, error) {
	if token == "" {
		return DiscogsLibrary{}, errors.InvalidArgumentError("Token is empty")
	}
	client, err := discogs.New(&discogs.Options{
		UserAgent: "Releases Watcher",
		Token:     string(token),
		URL:       "https://api.discogs.com", // optional
	})
	if err != nil {
		return DiscogsLibrary{}, err
	}
	return DiscogsLibrary{
		db:      db,
		cache:   cache,
		discogs: client,
		limiter: rate.NewLimiter(50*rate.Every(time.Minute), 1),
		cached:  &cached{},
	}, nil
}

func (l DiscogsLibrary) api() discogs.Discogs {
	l.limiter.Wait(context.Background())
	return l.discogs
}

func (l DiscogsLibrary) getRelease(releaseID int) (*discogs.Release, error) {
	id := strconv.Itoa(releaseID)
	freshness := 10 * 24 * time.Hour
	if l.cached.releases == nil {
		var err error
		l.cached.releases, err = GetAllCacheEntities[discogs.Release](l.cache, context.TODO(), "discogs_release", freshness)
		if err != nil {
			return nil, err
		}
		log.Infof("Loaded %d releases from cache", len(l.cached.releases))
	}
	if release, ok := l.cached.releases[id]; ok {
		log.Tracef("Loaded release %d from cache", releaseID)
		return release, nil
	}
	return GetCached(l.cache, context.TODO(), "discogs_release", id,
		freshness, func() (*discogs.Release, error) {
			return l.api().Release(releaseID)
		})
}

func (l DiscogsLibrary) getArtistID(artist string) (int, error) {
	search, err := GetCached(l.cache, context.TODO(), "discogs_artist_search", artist, 10*24*time.Hour, func() (*discogs.Search, error) {
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

func (l DiscogsLibrary) getArtistReleases(artistID int, page int) (*discogs.ArtistReleases, error) {
	id := fmt.Sprintf("%d_%d", artistID, page)
	return GetCached(l.cache, context.TODO(), "discord_artist_releases",
		id, 10*24*time.Hour, func() (*discogs.ArtistReleases, error) {
			return l.api().ArtistReleases(artistID,
				&discogs.Pagination{Page: page, PerPage: 500, Sort: "year", SortOrder: "asc"})
		})
}

func (l DiscogsLibrary) getReleases(artist string) ([]discogs.Release, error) {
	artistID, err := l.getArtistID(artist)
	if err != nil {
		return nil, err
	}
	releases := make([]discogs.Release, 0)
	page := 0
	for {
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
				if isCompilation(release) {
					continue
				}
				properType := isAlbum(release) || isEP(release) || isSingle(release)
				mainArtist := isMainArtist(release, artistID)
				if properType && mainArtist {
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

// GetActualAlbumsForArtists получает актуальные альбомы для списка артистов
func (l DiscogsLibrary) GetActualAlbumsForArtists(ctx context.Context, artists []string, out chan<- sqlc.ActualAlbum) {
	defer close(out)
	for i, artist := range artists {
		log.Tracef("Fetching for %s [%d of %d]", artist, i+1, len(artists))
		releases, err := l.getReleases(artist)
		if err != nil {
			log.Errorf("Error when processing artist '%v': %v", artist, err)
			continue
		}
		for _, release := range releases {
			kind := getKind(release)
			if kind == "" {
				log.Tracef("Unknown kind of release %v, skipped", release)
				continue
			}
			if isSoundtrack(release) {
				log.Tracef("Release %v is a soundtrack, skipped", release)
				continue
			}
			year := int32(release.Year)
			actualAlbum := sqlc.ActualAlbum{
				ID:     fmt.Sprint(release.ID),
				Artist: &artist,
				Name:   &release.Title,
				Year:   &year,
				Kind:   &kind,
			}
			out <- actualAlbum
		}
	}
}

func isReleaseType(release *discogs.Release, releaseType string) bool {
	for _, format := range release.Formats {
		for _, desc := range format.Descriptions {
			if strings.ToLower(desc) == releaseType {
				return true
			}
		}
	}
	return false
}

func isAlbum(release *discogs.Release) bool {
	return isReleaseType(release, "album")
}

func isEP(release *discogs.Release) bool {
	return isReleaseType(release, "ep")
}

func isSingle(release *discogs.Release) bool {
	return isReleaseType(release, "single")
}

func isCompilation(release *discogs.Release) bool {
	return isReleaseType(release, "compilation")
}

func isMainArtist(release *discogs.Release, artistID int) bool {
	return release.Artists[0].ID == artistID
}

func isSoundtrack(release discogs.Release) bool {
	return slices.Contains(release.Styles, "Soundtrack")
}

func getKind(release discogs.Release) string {
	if isAlbum(&release) {
		return "album"
	}
	if isSingle(&release) {
		return "single"
	}
	if isEP(&release) {
		return "EP"
	}
	return ""
}
