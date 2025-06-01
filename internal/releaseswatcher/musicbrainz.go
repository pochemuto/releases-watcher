package releaseswatcher

import (
	"context"
	"fmt"
	"time"

	"go.uploadedlobster.com/musicbrainzws2"
	mbtypes "go.uploadedlobster.com/mbtypes"
	"github.com/pochemuto/releases-watcher/sqlc"
	"golang.org/x/time/rate"
)

type MusicBrainzLibrary struct {
	db      DB
	cache   Cache
	mb      *musicbrainzws2.Client
	limiter *rate.Limiter
}

type MusicBrainzToken string

func NewMusicBrainzLibrary(token MusicBrainzToken, db DB, cache Cache) (MusicBrainzLibrary, error) {
	if token == "" {
		return MusicBrainzLibrary{}, fmt.Errorf("token is empty")
	}
	appInfo := musicbrainzws2.AppInfo{Name: "Releases Watcher", Version: "1.0"}
	mb := musicbrainzws2.NewClient(appInfo)
	mb.SetAuthToken(string(token))
	return MusicBrainzLibrary{
		db:      db,
		cache:   cache,
		mb:      mb,
		limiter: rate.NewLimiter(50*rate.Every(time.Minute), 1),
	}, nil
}

func (l MusicBrainzLibrary) api() *musicbrainzws2.Client {
	l.limiter.Wait(context.Background())
	return l.mb
}

func (l MusicBrainzLibrary) getRelease(releaseID string) (*musicbrainzws2.Release, error) {
	freshness := 10 * 24 * time.Hour
	return GetCached(l.cache, context.TODO(), "musicbrainz_release", releaseID, freshness, func() (*musicbrainzws2.Release, error) {
		release, err := l.api().LookupRelease(context.TODO(), mbtypes.MBID(releaseID), musicbrainzws2.IncludesFilter{})
		if err != nil {
			return nil, err
		}
		return &release, nil
	})
}

func (l MusicBrainzLibrary) getArtistID(artist string) (string, error) {
	freshness := 10 * 24 * time.Hour
	result, err := GetCached(l.cache, context.TODO(), "musicbrainz_artist_search", artist, freshness, func() (*musicbrainzws2.SearchArtistsResult, error) {
		filter := musicbrainzws2.SearchFilter{Query: artist}
		res, err := l.api().SearchArtists(context.TODO(), filter, musicbrainzws2.DefaultPaginator())
		if err != nil {
			return nil, err
		}
		return &res, nil
	})
	if err != nil {
		return "", err
	}
	if len(result.Artists) == 0 {
		return "", fmt.Errorf("artist '%s' not found", artist)
	}
	return string(result.Artists[0].ID), nil
}

func (l MusicBrainzLibrary) getArtistReleaseGroups(artistID string, offset int) (*musicbrainzws2.BrowseReleaseGroupsResult, error) {
	freshness := 10 * 24 * time.Hour
	cacheKey := fmt.Sprintf("%s_%d", artistID, offset)
	return GetCached(l.cache, context.TODO(), "musicbrainz_artist_releasegroups", cacheKey, freshness, func() (*musicbrainzws2.BrowseReleaseGroupsResult, error) {
		filter := musicbrainzws2.ReleaseGroupFilter{ArtistMBID: mbtypes.MBID(artistID)}
		paginator := musicbrainzws2.DefaultPaginator()
		paginator.Offset = offset
		paginator.Limit = 100
		res, err := l.api().BrowseReleaseGroups(context.TODO(), filter, paginator)
		if err != nil {
			return nil, err
		}
		return &res, nil
	})
}

func (l MusicBrainzLibrary) getReleases(artist string) ([]musicbrainzws2.Release, error) {
	artistID, err := l.getArtistID(artist)
	if err != nil {
		return nil, err
	}
	releases := make([]musicbrainzws2.Release, 0)
	offset := 0
	for {
		resp, err := l.getArtistReleaseGroups(artistID, offset)
		if err != nil {
			return nil, err
		}
		for _, rg := range resp.ReleaseGroups {
			if rg.PrimaryType == "Album" || rg.PrimaryType == "EP" || rg.PrimaryType == "Single" {
				// Get the first release for the group
				if len(rg.Releases) > 0 {
					releaseID := string(rg.Releases[0].ID)
					release, err := l.getRelease(releaseID)
					if err != nil {
						continue
					}
					releases = append(releases, *release)
				}
			}
		}
		offset += len(resp.ReleaseGroups)
		if offset >= resp.Count {
			break
		}
	}
	return releases, nil
}

func (l MusicBrainzLibrary) GetActualAlbumsForArtists(artists []string) ([]sqlc.ActualAlbum, error) {
	var actualAlbums []sqlc.ActualAlbum
	for _, artist := range artists {
		releases, err := l.getReleases(artist)
		if err != nil {
			continue
		}
		for _, release := range releases {
			kind := ""
			if release.ReleaseGroup != nil {
				kind = release.ReleaseGroup.PrimaryType
			}
			year := int32(0)
			if release.Date.Year > 0 {
				year = int32(release.Date.Year)
			}
			actualAlbum := sqlc.ActualAlbum{
				ID:     0, // MusicBrainz IDs are strings, adapt as needed
				Artist: &artist,
				Name:   &release.Title,
				Year:   &year,
				Kind:   &kind,
			}
			actualAlbums = append(actualAlbums, actualAlbum)
		}
	}
	return actualAlbums, nil
}
