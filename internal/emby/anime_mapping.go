package emby

import (
	"fmt"
	"math"

	"github.com/flaksp/anime365-sidecar/internal/animemapping"
	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/embyclient"
)

const (
	embyProviderIDIMDb             = "Imdb"
	embyProviderIDTheMovieDatabase = "Tmdb"
	embyProviderIDTheTVDB          = "Tvdb"
)

func (s *Service) applyAnimeMappingExternalIDs(
	embyItem *embyclient.BaseItemDto,
	myAnimeListID show.MyAnimeListID,
) bool {
	if s.animeMappingService == nil {
		return false
	}

	externalIDs, exists := s.animeMappingService.LookupByMyAnimeListID(int64(myAnimeListID))
	if !exists {
		return false
	}

	return applyExternalProviderIDs(embyItem, externalIDs)
}

func applyExternalProviderIDs(embyItem *embyclient.BaseItemDto, externalIDs animemapping.ExternalIDs) bool {
	if externalIDs.IMDb == "" && externalIDs.TheMovieDatabase == "" && externalIDs.TheTVDB == "" {
		return false
	}

	if embyItem.ProviderIds == nil {
		embyItem.ProviderIds = new(make(embyclient.ProviderIdDictionary))
	}

	if *embyItem.ProviderIds == nil {
		*embyItem.ProviderIds = make(embyclient.ProviderIdDictionary)
	}

	providerIDs := *embyItem.ProviderIds
	changed := false

	for provider, value := range map[string]string{
		embyProviderIDIMDb:             externalIDs.IMDb,
		embyProviderIDTheMovieDatabase: externalIDs.TheMovieDatabase,
		embyProviderIDTheTVDB:          externalIDs.TheTVDB,
	} {
		if value == "" || providerIDs[provider] == value {
			continue
		}

		providerIDs[provider] = value
		changed = true
	}

	return changed
}

func (s *Service) applyEpisodeSeasonNumber(
	item *embyclient.BaseItemDto,
	showID show.Anime365SeriesID,
	episodeEntity episode.Episode,
) (bool, error) {
	if s.animeMappingService == nil || episodeEntity.IsSpecial || episodeEntity.IsTrailer {
		return false, nil
	}

	malID, exists := s.manifestService.GetShowMyAnimeListID(showID)
	if !exists {
		return false, errShowNotFoundInManifest
	}

	mapping, exists := s.animeMappingService.LookupByMyAnimeListID(int64(malID))
	if !exists || mapping.SeasonNumber <= 0 {
		return false, nil
	}

	if mapping.SeasonNumber > math.MaxInt32 {
		return false, fmt.Errorf("season number %d is out of range int32", mapping.SeasonNumber)
	}

	seasonNumber := int32(mapping.SeasonNumber)
	if item.ParentIndexNumber != nil && *item.ParentIndexNumber == seasonNumber {
		return false, nil
	}

	item.ParentIndexNumber = new(seasonNumber)

	return true, nil
}
