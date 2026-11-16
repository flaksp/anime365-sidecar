package emby

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"slices"

	"github.com/flaksp/anime365-sidecar/internal/mediaserver"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/embyclient"
)

// UpdateSeasonMetadata sets the main season name and number using Fribb mappings.
// The library layout keeps each anime in Season 01, with specials in Season 00.
func (s *Service) UpdateSeasonMetadata(ctx context.Context, showEntity show.Show) error {
	seasonNumber := int64(1)

	name := showEntity.TypeLabel
	if s.animeMappingService != nil {
		mapping, exists := s.animeMappingService.LookupByMyAnimeListID(int64(showEntity.MyAnimeListID))
		if exists && mapping.SeasonNumber > 0 {
			seasonNumber = mapping.SeasonNumber
			name = fmt.Sprintf("%d сезон", seasonNumber)
		}
	}

	if seasonNumber > math.MaxInt32 {
		return fmt.Errorf("season number %d is out of range int32", seasonNumber)
	}

	series, err := s.getSeriesItem(ctx, showEntity.Anime365ID)
	if err != nil {
		return err
	}

	showPath, err := s.getEmbyShowPath(showEntity.Anime365ID)
	if err != nil {
		return err
	}

	seasonPath := filepath.Join(showPath, "Season 01")

	result, err := getEmbyJSONResponse[embyclient.QueryResultBaseItemDto](
		s.embyClient.GetItemsWithResponse(ctx, &embyclient.GetItemsParams{
			ParentId:         series.Id,
			IncludeItemTypes: new("Season"),
			Recursive:        new(false),
			Fields:           new(embyItemFields),
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to find season items: %w", err)
	}

	for _, item := range result.Items {
		if filepath.Clean(value(item.Path)) != filepath.Clean(seasonPath) {
			continue
		}

		nameLocked := slices.Contains(item.LockedFields, embyclient.Name)

		if (name != "" && (value(item.Name) != name || !nameLocked)) || value(item.IndexNumber) != int32(seasonNumber) {
			if name != "" {
				item.Name = new(name)
			}

			item.IndexNumber = new(int32(seasonNumber))
			if name != "" && !nameLocked {
				item.LockedFields = append(item.LockedFields, embyclient.Name)
			}

			if err := s.updateItem(ctx, item); err != nil {
				return fmt.Errorf("failed to update season metadata: %w", err)
			}
		}

		return nil
	}

	return mediaserver.ErrItemNotFound
}
