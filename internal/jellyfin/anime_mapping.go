package jellyfin

import (
	"fmt"
	"math"

	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/jellyfinclient"
)

func (s *Service) applyAnimeMappingExternalIDs(item *jellyfinclient.BaseItemDto, malID show.MyAnimeListID) bool {
	if s.animeMappingService == nil {
		return false
	}

	ids, exists := s.animeMappingService.LookupByMyAnimeListID(int64(malID))
	if !exists {
		return false
	}

	if item.ProviderIds == nil {
		item.ProviderIds = make(map[string]*string)
	}

	changed := false

	for provider, id := range map[string]string{"Imdb": ids.IMDb, "Tmdb": ids.TheMovieDatabase, "Tvdb": ids.TheTVDB} {
		if id != "" && value(item.ProviderIds[provider]) != id {
			item.ProviderIds[provider] = new(id)
			changed = true
		}
	}

	return changed
}

func (s *Service) applyEpisodeSeasonNumber(
	item *jellyfinclient.BaseItemDto,
	showID show.Anime365SeriesID,
	episodeEntity episode.Episode,
) (bool, error) {
	if episodeEntity.IsSpecial || episodeEntity.IsTrailer {
		return false, nil
	}

	malID, exists := s.manifestService.GetShowMyAnimeListID(showID)
	if !exists {
		return false, errShowNotFoundInManifest
	}

	number := int64(1)

	if s.animeMappingService != nil {
		mapping, found := s.animeMappingService.LookupByMyAnimeListID(int64(malID))
		if found && mapping.SeasonNumber > 0 {
			number = mapping.SeasonNumber
		}
	}

	if number > math.MaxInt32 {
		return false, fmt.Errorf("season number %d is out of range int32", number)
	}

	seasonNumber := int32(number)
	if item.ParentIndexNumber != nil && *item.ParentIndexNumber == seasonNumber {
		return false, nil
	}

	item.ParentIndexNumber = new(seasonNumber)

	return true, nil
}
