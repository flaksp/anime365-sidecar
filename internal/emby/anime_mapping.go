package emby

import (
	"github.com/flaksp/anime365-sidecar/internal/animemapping"
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
		embyItem.ProviderIds = new(map[string]string)
	}

	if *embyItem.ProviderIds == nil {
		*embyItem.ProviderIds = make(map[string]string)
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
