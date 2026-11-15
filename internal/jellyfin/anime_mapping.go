package jellyfin

import (
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
