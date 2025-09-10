package releaseswatcher

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/pochemuto/releases-watcher/sqlc"
	mbtypes "go.uploadedlobster.com/mbtypes"
	"go.uploadedlobster.com/musicbrainzws2"
	"golang.org/x/time/rate"
)

type MusicBrainzLibrary struct {
	db      DB
	cache   Cache
	mb      *musicbrainzws2.Client
	limiter *rate.Limiter
}

type MusicBrainzToken string

func days(d int) time.Duration {
	return time.Duration(d) * 24 * time.Hour
}

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

func (l MusicBrainzLibrary) Name() string {
	return "MusicBrainz"
}

func (l MusicBrainzLibrary) api() *musicbrainzws2.Client {
	l.limiter.Wait(context.Background())
	return l.mb
}

func (l MusicBrainzLibrary) getRelease(releaseID string) (*musicbrainzws2.Release, error) {
	freshness := days(7)
	return GetCached(l.cache, context.TODO(), "musicbrainz_release", releaseID, freshness, func() (*musicbrainzws2.Release, error) {
		release, err := l.api().LookupRelease(context.TODO(), mbtypes.MBID(releaseID), musicbrainzws2.IncludesFilter{Includes: []string{"release-groups"}})
		if err != nil {
			return nil, err
		}
		return &release, nil
	})
}

func (l MusicBrainzLibrary) getArtistID(artist string) (string, error) {
	freshness := days(90)
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
	freshness := days(7)
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
		log.Infof("Found %d release groups\n", len(res.ReleaseGroups))
		return &res, nil
	})
}

func (l MusicBrainzLibrary) getArtistReleaseGroup(releaseGroupID mbtypes.MBID) (*musicbrainzws2.ReleaseGroup, error) {
	freshness := days(30)
	return GetCached(l.cache, context.TODO(), "musicbrainz_releasegroup", string(releaseGroupID), freshness, func() (*musicbrainzws2.ReleaseGroup, error) {
		releaseGroup, err := l.api().LookupReleaseGroup(context.TODO(), releaseGroupID, musicbrainzws2.IncludesFilter{Includes: []string{"releases"}})
		if err != nil {
			return nil, err
		}
		return &releaseGroup, nil
	})
}

func (l MusicBrainzLibrary) getReleases(artist string, out chan<- musicbrainzws2.Release) {
	defer close(out)
	var excludedSecondaryTypes = []string{"Compilation", "Live", "Remix", "Demo", "Mixtape/Street", "Bootleg", "Promotion", "Withdrawn", "Expunged", "Pseudo-Release", "Accepted"}
	var excludedReleaseStatuses = []string{"Bootleg"}
	artistID, err := l.getArtistID(artist)
	if err != nil {
		log.Errorf("Error getting artist ID for %s: %v", artist, err)
		return
	}
	offset := 0
	for {
		log.Infof("Checking releases for artist %s (offset %d)\n", artist, offset)
		resp, err := l.getArtistReleaseGroups(artistID, offset)
		if err != nil {
			log.Errorf("Error getting release groups for artist %s: %v", artist, err)
			return
		}
		for _, rg := range resp.ReleaseGroups {
			if rg.PrimaryType == "Album" || rg.PrimaryType == "EP" || rg.PrimaryType == "Single" {
				if rg.SecondaryTypes != nil && slices.ContainsFunc(rg.SecondaryTypes, func(s string) bool {
					return slices.Contains(excludedSecondaryTypes, s)
				}) {
					continue
				}
				secondaryTypes := ""
				if len(rg.SecondaryTypes) > 0 {
					secondaryTypes = fmt.Sprintf(" (%s)", strings.Join(rg.SecondaryTypes, ", "))
				}
				log.Infof("  Getting release for [%s%s] %s\n", rg.PrimaryType, secondaryTypes, rg.Title)
				rg, err := l.getArtistReleaseGroup(rg.ID)
				if err != nil {
					log.Errorf("Error getting release group %s: %v", rg.ID, err)
				}
				// Get the first release for the group
				if len(rg.Releases) > 0 {
					releaseID := string(rg.Releases[0].ID)
					release, err := l.getRelease(releaseID)
					if err != nil {
						continue
					}
					if slices.Contains(excludedReleaseStatuses, release.Status) {
						continue
					}
					out <- *release
				}
			}
		}
		offset += len(resp.ReleaseGroups)
		if offset >= resp.Count {
			break
		}
	}
}

func (l MusicBrainzLibrary) GetActualAlbumsForArtists(ctx context.Context, artists []string, out chan<- sqlc.ActualAlbum) {
	defer close(out)
	for i, artist := range artists {
		log.Infof("Processing artist %d of %d: %s", i+1, len(artists), artist)

		releases := make(chan musicbrainzws2.Release)
		go l.getReleases(artist, releases)
		for release := range releases {
			kind := ""
			if release.ReleaseGroup != nil {
				kind = release.ReleaseGroup.PrimaryType
			}
			year := int32(0)
			if release.Date.Year > 0 {
				year = int32(release.Date.Year)
			}
			actualAlbum := sqlc.ActualAlbum{
				ID:     string(release.ID),
				Artist: &artist,
				Name:   &release.Title,
				Year:   &year,
				Kind:   &kind,
			}
			out <- actualAlbum
		}
	}
}
