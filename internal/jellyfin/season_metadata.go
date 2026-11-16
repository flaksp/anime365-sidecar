package jellyfin

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"slices"

	"github.com/flaksp/anime365-sidecar/internal/mediaserver"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/jellyfinclient"
)

// UpdateSeasonMetadata sets the main season name and number using Fribb mappings.
// The library layout keeps each anime in Season 01, with specials in Season 00.
func (s *Service) UpdateSeasonMetadata(ctx context.Context, showEntity show.Show) error {
	if s.animeMappingService == nil {
		return nil
	}

	mapping, exists := s.animeMappingService.LookupByMyAnimeListID(int64(showEntity.MyAnimeListID))
	if !exists || mapping.SeasonNumber <= 0 {
		return nil
	}

	if mapping.SeasonNumber > math.MaxInt32 {
		return fmt.Errorf("season number %d is out of range int32", mapping.SeasonNumber)
	}

	series, err := s.getSeriesItem(ctx, showEntity.Anime365ID)
	if err != nil {
		return err
	}

	showPath, err := s.getJellyfinShowPath(showEntity.Anime365ID)
	if err != nil {
		return err
	}

	seasonPath := filepath.Join(showPath, "Season 01")

	items, err := s.getItems(ctx, jellyfinclient.GetItemsParams{
		ParentId:         series.Id,
		IncludeItemTypes: []jellyfinclient.BaseItemKind{jellyfinclient.BaseItemKindSeason},
		Recursive:        new(false),
	})
	if err != nil {
		return fmt.Errorf("failed to find season items: %w", err)
	}

	for _, item := range items {
		if filepath.Clean(value(item.Path)) != filepath.Clean(seasonPath) {
			continue
		}

		name := fmt.Sprintf("%d сезон", mapping.SeasonNumber)

		nameLocked := slices.Contains(item.LockedFields, jellyfinclient.MetadataFieldName)
		if value(item.Name) == name && value(item.IndexNumber) == int32(mapping.SeasonNumber) && nameLocked {
			return nil
		}

		item.Name = new(name)

		item.IndexNumber = new(int32(mapping.SeasonNumber))
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
