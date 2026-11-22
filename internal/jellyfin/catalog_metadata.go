package jellyfin

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

	nameOrDateChanged := metadata.ApplyTitleAndDate(&item.Name, &item.PremiereDate)
	tags := metadata.ApplyTags(item.Tags)
	tagsChanged := !slices.Equal(item.Tags, tags)
	item.Tags = tags

	if !nameOrDateChanged && !tagsChanged {
		return nil
	}

	return s.updateItem(ctx, item)
}
