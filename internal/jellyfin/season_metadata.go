package jellyfin

import (
	"context"
	"fmt"
	"slices"

	"github.com/flaksp/anime365-sidecar/internal/mediaserver"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/jellyfinclient"
)

// UpdateSeasonMetadata names the main season using Fribb's mapped season number.
// The library layout keeps each anime in Season 01, with specials in Season 00.
func (s *Service) UpdateSeasonMetadata(ctx context.Context, showEntity show.Show) error {
	if s.animeMappingService == nil {
		return nil
	}

	mapping, exists := s.animeMappingService.LookupByMyAnimeListID(int64(showEntity.MyAnimeListID))
	if !exists || mapping.SeasonNumber <= 0 {
		return nil
	}

	series, err := s.getSeriesItem(ctx, showEntity.Anime365ID)
	if err != nil {
		return err
	}

	items, err := s.getItems(ctx, jellyfinclient.GetItemsParams{
		ParentId:         series.Id,
		IncludeItemTypes: []jellyfinclient.BaseItemKind{jellyfinclient.BaseItemKindSeason},
		Recursive:        new(false),
	})
	if err != nil {
		return fmt.Errorf("failed to find season items: %w", err)
	}

	for _, item := range items {
		if value(item.IndexNumber) != 1 {
			continue
		}

		name := fmt.Sprintf("%d сезон", mapping.SeasonNumber)

		nameLocked := slices.Contains(item.LockedFields, jellyfinclient.MetadataFieldName)
		if value(item.Name) == name && nameLocked {
			return nil
		}

		item.Name = new(name)
		if !nameLocked {
			item.LockedFields = append(item.LockedFields, jellyfinclient.MetadataFieldName)
		}

		if err := s.updateItem(ctx, item); err != nil {
			return fmt.Errorf("failed to update season metadata: %w", err)
		}

		return nil
	}

	return mediaserver.ErrItemNotFound
}
