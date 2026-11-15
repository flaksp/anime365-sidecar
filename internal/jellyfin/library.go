package jellyfin

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/mediaserver"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/jellyfinclient"
	"github.com/google/uuid"
)

func (s *Service) detectLibraryDirectory(ctx context.Context, libraryID uuid.UUID) error {
	folders, err := getJellyfinJSONResponse[[]jellyfinclient.VirtualFolderInfo](
		s.jellyfinClient.GetVirtualFoldersWithResponse(ctx),
	)
	if err != nil {
		return err
	}

	for _, folder := range folders {
		id, err := uuid.Parse(value(folder.ItemId))
		if err != nil || id != libraryID {
			continue
		}

		if len(folder.Locations) != 1 {
			return fmt.Errorf("Jellyfin library %s must have exactly one location", libraryID)
		}

		s.jellyfinLibraryItemID = id
		s.jellyfinLibraryRootDirectory = folder.Locations[0]
		s.logger.InfoContext(
			ctx,
			"Detected Jellyfin library root",
			slog.String("library_id", id.String()),
			slog.String("library_root_directory", folder.Locations[0]),
		)

		return nil
	}

	return fmt.Errorf("Jellyfin library %s does not exist", libraryID)
}

// getItems follows pagination instead of relying on a server-specific default limit.
func (s *Service) getItems(
	ctx context.Context,
	params jellyfinclient.GetItemsParams,
) ([]jellyfinclient.BaseItemDto, error) {
	params.UserId = &s.jellyfinUserID
	params.EnableUserData = new(true)
	params.EnableTotalRecordCount = new(true)
	params.Fields = itemFields
	params.Limit = new(int32(200))

	var items []jellyfinclient.BaseItemDto

	for start := int32(0); ; {
		params.StartIndex = &start

		result, err := getJellyfinJSONResponse[jellyfinclient.BaseItemDtoQueryResult](
			s.jellyfinClient.GetItemsWithResponse(ctx, &params),
		)
		if err != nil {
			return nil, err
		}

		items = append(items, result.Items...)

		start += int32(len(result.Items))
		if len(result.Items) == 0 || (result.TotalRecordCount != nil && start >= *result.TotalRecordCount) {
			break
		}
	}

	return items, nil
}

var itemFields = []jellyfinclient.ItemFields{
	jellyfinclient.ItemFieldsPath, jellyfinclient.ItemFieldsMediaSources,
	jellyfinclient.ItemFieldsMediaStreams, jellyfinclient.ItemFieldsOverview,
	jellyfinclient.ItemFieldsGenres, jellyfinclient.ItemFieldsPeople,
	jellyfinclient.ItemFieldsProviderIds, jellyfinclient.ItemFieldsSortName,
	jellyfinclient.ItemFieldsStudios, jellyfinclient.ItemFieldsTaglines,
	jellyfinclient.ItemFieldsTags, jellyfinclient.ItemFieldsOriginalTitle,
	jellyfinclient.ItemFieldsDateCreated, jellyfinclient.ItemFieldsParentId,
	jellyfinclient.ItemFieldsSettings,
}

func (s *Service) getSeriesItem(ctx context.Context, showID show.Anime365SeriesID) (jellyfinclient.BaseItemDto, error) {
	path, err := s.getJellyfinShowPath(showID)
	if err != nil {
		return jellyfinclient.BaseItemDto{}, err
	}

	if cachedID, exists := s.manifestService.GetShowItemID(showID); exists {
		item, found, err := s.getCachedItem(ctx, cachedID, path)
		if err != nil {
			return jellyfinclient.BaseItemDto{}, err
		}

		if found {
			return item, nil
		}
	}

	return s.findItemByPath(ctx, path, jellyfinclient.BaseItemKindSeries, s.jellyfinLibraryItemID)
}

func (s *Service) getEpisodeItem(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
) (jellyfinclient.BaseItemDto, error) {
	path, err := s.getJellyfinTranslationPath(showID, episodeID, translationID)
	if err != nil {
		return jellyfinclient.BaseItemDto{}, err
	}

	if cachedID, exists := s.manifestService.GetTranslationItemID(showID, episodeID, translationID); exists {
		item, found, err := s.getCachedItem(ctx, cachedID, path)
		if err != nil {
			return jellyfinclient.BaseItemDto{}, err
		}

		if found {
			return item, nil
		}
	}

	series, err := s.getSeriesItem(ctx, showID)
	if err != nil {
		return jellyfinclient.BaseItemDto{}, err
	}

	return s.findItemByPath(ctx, path, jellyfinclient.BaseItemKindEpisode, value(series.Id))
}

// Cached provider IDs avoid rescanning a whole library for each metadata update.
// Verify paths on every read so stale IDs cannot target a different file.
func (s *Service) getCachedItem(ctx context.Context, cachedID, path string) (jellyfinclient.BaseItemDto, bool, error) {
	id, err := uuid.Parse(cachedID)
	if err != nil || id == uuid.Nil {
		return jellyfinclient.BaseItemDto{}, false, nil //nolint:nilerr // Invalid cached IDs fall back to a path lookup.
	}

	items, err := s.getItems(ctx, jellyfinclient.GetItemsParams{Ids: []uuid.UUID{id}})
	if err != nil {
		return jellyfinclient.BaseItemDto{}, false, err
	}

	for _, item := range items {
		if value(item.Id) != id {
			continue
		}

		for _, itemPath := range itemPaths(item) {
			if filepath.Clean(itemPath) == filepath.Clean(path) {
				return item, true, nil
			}
		}
	}

	return jellyfinclient.BaseItemDto{}, false, nil
}

// Jellyfin has no Path query filter. Match both primary and alternate version paths.
func (s *Service) findItemByPath(
	ctx context.Context,
	path string,
	kind jellyfinclient.BaseItemKind,
	parentID uuid.UUID,
) (jellyfinclient.BaseItemDto, error) {
	items, err := s.getItems(
		ctx,
		jellyfinclient.GetItemsParams{
			ParentId:         &parentID,
			Recursive:        new(true),
			IncludeItemTypes: []jellyfinclient.BaseItemKind{kind},
		},
	)
	if err != nil {
		return jellyfinclient.BaseItemDto{}, err
	}

	for _, item := range items {
		for _, itemPath := range itemPaths(item) {
			if filepath.Clean(itemPath) == filepath.Clean(path) {
				return item, nil
			}
		}
	}

	return jellyfinclient.BaseItemDto{}, mediaserver.ErrItemNotFound
}

func itemPaths(item jellyfinclient.BaseItemDto) []string {
	var paths []string
	if value(item.Path) != "" {
		paths = append(paths, *item.Path)
	}

	for _, source := range item.MediaSources {
		if value(source.Path) != "" {
			paths = append(paths, *source.Path)
		}
	}

	return paths
}

func (s *Service) GetIDsWithoutMetadata(
	ctx context.Context,
) (map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{}, error) {
	return s.getIDsByMetadata(ctx, false)
}

func (s *Service) GetIDsWithMetadata(
	ctx context.Context,
) (map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{}, error) {
	return s.getIDsByMetadata(ctx, true)
}

func (s *Service) getIDsByMetadata(
	ctx context.Context,
	withMetadata bool,
) (map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{}, error) {
	items, err := s.getItems(
		ctx,
		jellyfinclient.GetItemsParams{
			ParentId:         &s.jellyfinLibraryItemID,
			Recursive:        new(true),
			IncludeItemTypes: []jellyfinclient.BaseItemKind{jellyfinclient.BaseItemKindEpisode},
		},
	)
	if err != nil {
		return nil, err
	}

	result := make(map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{})

	for _, item := range items {
		for _, path := range itemPaths(item) {
			relative, err := filepath.Rel(s.jellyfinLibraryRootDirectory, path)
			if err != nil {
				return nil, err
			}

			showID, episodeID, translationID, exists := s.manifestService.GetIDsByVideoFileRelativePath(relative)
			if !exists {
				continue
			}

			id, initialized := s.manifestService.GetTranslationItemID(showID, episodeID, translationID)
			// Re-indexed items need their initial metadata applied again.
			initialized = initialized && id == value(item.Id).String()
			if initialized != withMetadata {
				continue
			}

			if result[showID] == nil {
				result[showID] = make(map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{})
			}

			if result[showID][episodeID] == nil {
				result[showID][episodeID] = make(map[episode.Anime365TranslationID]struct{})
			}

			result[showID][episodeID][translationID] = struct{}{}
		}
	}

	return result, nil
}

func (s *Service) GetLastWatchedEpisodeNumber(
	ctx context.Context,
	showID show.Anime365SeriesID,
) (int64, episode.Anime365TranslationID, error) {
	series, err := s.getSeriesItem(ctx, showID)
	if err != nil {
		return 0, 0, err
	}

	items, err := s.getItems(
		ctx,
		jellyfinclient.GetItemsParams{
			ParentId:         series.Id,
			Recursive:        new(true),
			IsPlayed:         new(true),
			IncludeItemTypes: []jellyfinclient.BaseItemKind{jellyfinclient.BaseItemKindEpisode},
		},
	)
	if err != nil {
		return 0, 0, err
	}

	var (
		number int64
		lastID episode.Anime365TranslationID
	)

	for _, item := range items {
		for _, path := range itemPaths(item) {
			relative, err := filepath.Rel(s.jellyfinLibraryRootDirectory, path)
			if err != nil {
				return 0, 0, err
			}

			itemShowID, _, translationID, exists := s.manifestService.GetIDsByVideoFileRelativePath(relative)
			if exists && itemShowID == showID && int64(value(item.IndexNumber)) > number {
				number = int64(value(item.IndexNumber))
				lastID = translationID
			}
		}
	}

	if lastID == 0 {
		return 0, 0, mediaserver.ErrItemNotFound
	}

	return number, lastID, nil
}

func (s *Service) GetEpisodeMetadataForNotification(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
) (mediaserver.EpisodeMetadataForNotification, error) {
	item, err := s.getEpisodeItem(ctx, showID, episodeID, translationID)
	if err != nil {
		return mediaserver.EpisodeMetadataForNotification{}, err
	}

	path, err := s.getJellyfinTranslationPath(showID, episodeID, translationID)
	if err != nil {
		return mediaserver.EpisodeMetadataForNotification{}, err
	}

	webURL := *s.jellyfinPublicURL
	webURL.Path = strings.TrimRight(webURL.Path, "/") + "/web/index.html"
	webURL.RawPath = ""
	webURL.RawQuery = ""
	query := webURL.Query()
	query.Set("id", strings.ReplaceAll(value(item.Id).String(), "-", ""))
	query.Set("serverId", value(item.ServerId))
	webURL.Fragment = "/details?" + query.Encode()

	result := mediaserver.EpisodeMetadataForNotification{
		WebURL:       &webURL,
		SeriesName:   value(item.SeriesName),
		EpisodeLabel: value(item.Name),
	}
	for _, source := range item.MediaSources {
		if filepath.Clean(value(source.Path)) != filepath.Clean(path) {
			continue
		}

		result.Bitrate = int64(value(source.Bitrate))
		for _, stream := range source.MediaStreams {
			if stream.Type != nil && *stream.Type == jellyfinclient.MediaStreamTypeVideo {
				result.VideoMetadataDisplayTitle = value(stream.DisplayTitle)

				break
			}
		}
	}

	if result.Bitrate == 0 || result.VideoMetadataDisplayTitle == "" {
		return mediaserver.EpisodeMetadataForNotification{}, fmt.Errorf(
			"missing video metadata for Jellyfin translation %d",
			translationID,
		)
	}

	return result, nil
}
