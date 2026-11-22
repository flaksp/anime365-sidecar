package emby

import (
	"context"
	"fmt"
	"slices"

	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/show"
)

func (s *Service) UpdateTranslationCatalogMetadata(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
	metadata episode.CatalogMetadata,
) error {
	s.episodeMetadataMu.Lock()
	defer s.episodeMetadataMu.Unlock()

	item, err := s.getEpisodeItem(ctx, showID, episodeID, translationID)
	if err != nil {
		return fmt.Errorf("getting episode item: %w", err)
	}

	detailsChanged := metadata.ApplyDetails(&item.Name, &item.PremiereDate, &item.CommunityRating)

	tags := metadata.ApplyTags(item.Tags)

	tagsChanged := !slices.Equal(item.Tags, tags)
	if tagsChanged {
		item.Tags = tags
		item.TagItems = nil
	}

	if !detailsChanged && !tagsChanged {
		return nil
	}

	return s.updateItem(ctx, item)
}
