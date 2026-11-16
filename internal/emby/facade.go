package emby

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/flaksp/anime365-sidecar/internal/animemapping"
	"github.com/flaksp/anime365-sidecar/internal/emby/internal/manifest"
	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/libraryfiles"
	"github.com/flaksp/anime365-sidecar/internal/mediaserver"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/embyclient"
)

var _ mediaserver.Server = (*Service)(nil)

var (
	errShowNotFoundInManifest        = errors.New("show not found in manifest")
	errTranslationNotFoundInManifest = errors.New("translation not found in manifest")
)

func NewService(
	downloadsDirectory string,
	embyUserID string,
	libraryID string,
	logger *slog.Logger,
	embyClient *embyclient.ClientWithResponses,
	embyPublicURL *url.URL,
	animeMappingService *animemapping.Service,
) *Service {
	manifestService := manifest.NewService(downloadsDirectory, logger)

	return &Service{
		logger:              logger,
		manifestService:     manifestService,
		libraryID:           libraryID,
		Library:             libraryfiles.NewLibrary(downloadsDirectory, manifestService),
		embyClient:          embyClient,
		embyUserID:          embyUserID,
		embyPublicURL:       embyPublicURL,
		animeMappingService: animeMappingService,
	}
}

type Service struct {
	manifestService *manifest.Service
	libraryID       string
	logger          *slog.Logger
	*libraryfiles.Library
	embyClient               *embyclient.ClientWithResponses
	embyPublicURL            *url.URL
	animeMappingService      *animemapping.Service
	embyLibraryItemID        string
	embyLibraryRootDirectory string
	embyUserID               string
}

// Initialize loads adapter-owned state before discovering the configured library.
func (s *Service) Initialize(ctx context.Context) error {
	if err := s.manifestService.LoadFromDisk(ctx); err != nil {
		return err
	}

	return s.detectLibraryDirectory(ctx, s.libraryID)
}

func (s *Service) detectLibraryDirectory(ctx context.Context, libraryID string) error {
	virtualFolderDTOs, err := getEmbyJSONResponse[embyclient.QueryResultVirtualFolderInfo](
		s.embyClient.GetLibraryVirtualfoldersQueryWithResponse(ctx, nil),
	)
	if err != nil {
		return err
	}

	for _, virtualFolderDTO := range virtualFolderDTOs.Items {
		if value(virtualFolderDTO.ItemId) != libraryID {
			continue
		}

		if len(virtualFolderDTO.Locations) == 0 {
			return fmt.Errorf("expected library location list not empty for %s", value(virtualFolderDTO.Name))
		}

		s.embyLibraryItemID = value(virtualFolderDTO.ItemId)
		s.embyLibraryRootDirectory = virtualFolderDTO.Locations[0]

		s.logger.InfoContext(
			ctx,
			"Detected library root metadata",
			slog.String("library_item_name", value(virtualFolderDTO.Name)),
			slog.String("library_item_id", s.embyLibraryItemID),
			slog.String("library_root_directory", s.embyLibraryRootDirectory),
		)

		return nil
	}

	return fmt.Errorf("failed to detect library metadata, library with id \"%s\" does not exist", libraryID)
}

func (s *Service) RefreshLibrary(ctx context.Context) error {
	return embyResponseError(s.embyClient.PostLibraryRefreshWithResponse(ctx))
}

func (s *Service) DeleteTranslationIfNotPlaying(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
) (bool, error) {
	embyItemID, hasItemID := s.manifestService.GetTranslationItemID(
		showID,
		episodeID,
		translationID,
	)
	if hasItemID {
		sessions, err := getEmbyJSONResponse[[]embyclient.SessionSessionInfo](
			s.embyClient.GetSessionsWithResponse(ctx, nil),
		)
		if err != nil {
			return false, fmt.Errorf("get active Emby sessions: %w", err)
		}

		if _, isPlaying := getPlayingItemIDs(sessions)[embyItemID]; isPlaying {
			return false, nil
		}
	}

	if err := s.DeleteTranslation(showID, episodeID, translationID); err != nil {
		return false, err
	}

	return true, nil
}

func getPlayingItemIDs(sessions []embyclient.SessionSessionInfo) map[string]struct{} {
	playingItemIDs := make(map[string]struct{})

	for _, session := range sessions {
		if session.NowPlayingItem == nil || value(session.NowPlayingItem.Id) == "" {
			continue
		}

		playingItemIDs[value(session.NowPlayingItem.Id)] = struct{}{}
	}

	return playingItemIDs
}

func (s *Service) SaveTranslationPaths(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
	videoFileAbsolutePath string,
	subtitlesFileAbsolutePath string,
	height int64,
) error {
	videoFileRelativePath, err := filepath.Rel(s.Directory, videoFileAbsolutePath)
	if err != nil {
		return fmt.Errorf("failed to get video file relative path: %w", err)
	}

	var subtitlesFileRelativePath string

	if subtitlesFileAbsolutePath != "" {
		subtitlesFileRelativePath, err = filepath.Rel(s.Directory, subtitlesFileAbsolutePath)
		if err != nil {
			return fmt.Errorf("failed to get subtitles file relative path: %w", err)
		}
	}

	if err := s.manifestService.SetTranslationEntry(
		showID,
		episodeID,
		translationID,
		videoFileRelativePath,
		subtitlesFileRelativePath,
		height,
	); err != nil {
		return fmt.Errorf("failed to add translation entry to manifest: %w", err)
	}

	if err := s.RefreshLibrary(ctx); err != nil {
		s.logger.WarnContext(ctx, "Failed to refresh library, but it's not critical")
	}

	return nil
}

var oneOrMoreLineBreaksRegexp = regexp.MustCompile(`\n+`)

var showLockedFields = []embyclient.MetadataFields{
	embyclient.Name,
	embyclient.OriginalTitle,
	embyclient.CommunityRating,
	embyclient.Overview,
	embyclient.Genres,
	embyclient.SortName,
	embyclient.Studios,
	embyclient.Runtime,
	embyclient.OfficialRating,
	embyclient.Cast,
	embyclient.Tagline,
	embyclient.Tags,
}

// InitializeShowMetadata updates Emby item with metadata from Anime 365.
// If item not found in Emby library error mediaserver.ErrItemNotFound returned.
// If entry not found in manifest error errShowNotFoundInManifest returned.
func (s *Service) InitializeShowMetadata(
	ctx context.Context,
	showFromAnime365 show.Show,
) error {
	embyItem, err := s.getSeriesItem(ctx, showFromAnime365.Anime365ID)
	if err != nil {
		if errors.Is(err, mediaserver.ErrItemNotFound) {
			return mediaserver.ErrItemNotFound
		} else if errors.Is(err, errShowNotFoundInManifest) {
			return errShowNotFoundInManifest
		} else {
			return fmt.Errorf(
				"failed to get series emby item for show %d: %w",
				showFromAnime365.Anime365ID,
				err,
			)
		}
	}

	newName := showFromAnime365.TitleRomaji

	if showFromAnime365.TitleRussian != "" {
		newName = showFromAnime365.TitleRussian
	}

	embyItem.Name = new(newName)
	embyItem.SortName = new(newName)

	embyItem.OriginalTitle = new(showFromAnime365.TitleRomaji)

	if showFromAnime365.MyAnimeListScore > 0 {
		embyItem.CommunityRating = new(float32(showFromAnime365.MyAnimeListScore))
	}

	if showFromAnime365.Year > 0 {
		embyItem.ProductionYear = new(int32(showFromAnime365.Year))
	}

	if showFromAnime365.Description != "" {
		embyItem.Overview = new(oneOrMoreLineBreaksRegexp.ReplaceAllString(showFromAnime365.Description, "\n\n"))
	}

	if showFromAnime365.TypeLabel != "" && showFromAnime365.SeasonLabel != "" {
		embyItem.Taglines = []string{fmt.Sprintf("%s, %s", showFromAnime365.TypeLabel, showFromAnime365.SeasonLabel)}
		embyItem.Tags = []string{showFromAnime365.TypeLabel, showFromAnime365.SeasonLabel}
	} else if showFromAnime365.SeasonLabel != "" {
		embyItem.Taglines = []string{showFromAnime365.SeasonLabel}
		embyItem.Tags = []string{showFromAnime365.SeasonLabel}
	} else if showFromAnime365.TypeLabel != "" {
		embyItem.Taglines = []string{showFromAnime365.TypeLabel}
		embyItem.Tags = []string{showFromAnime365.TypeLabel}
	}

	if len(showFromAnime365.Genres) > 0 {
		embyItem.Genres = showFromAnime365.Genres
	}

	if len(showFromAnime365.EpisodePreviews) > 0 {
		if showFromAnime365.IsOngoing {
			embyItem.Status = new("Continuing")
		} else {
			embyItem.Status = new("Ended")
		}
	}

	s.applyAnimeMappingExternalIDs(&embyItem, showFromAnime365.MyAnimeListID)

	embyItem.DisplayOrder = new("absolute")

	// Keep sidecar-owned textual metadata protected with field locks.
	embyItem.LockedFields = showLockedFields

	if err := s.updateItem(ctx, embyItem); err != nil {
		return fmt.Errorf("failed to update show item %s: %w", value(embyItem.Id), err)
	}

	if err := embyResponseError(
		s.embyClient.PostItemsByIdRefreshWithBodyWithResponse(
			ctx,
			value(embyItem.Id),
			&embyclient.PostItemsByIdRefreshParams{
				Recursive: new(
					false,
				),
				MetadataRefreshMode: new(embyclient.MetadataRefreshModeFullRefresh),
				ImageRefreshMode:    new(embyclient.MetadataRefreshModeFullRefresh),
				ReplaceAllMetadata:  new(false),
				ReplaceAllImages:    new(false),
			},
			"application/json",
			nil,
		),
	); err != nil {
		return fmt.Errorf("failed to refresh missing images for show item %s: %w", value(embyItem.Id), err)
	}

	if err := s.manifestService.SetShowItemID(showFromAnime365.Anime365ID, value(embyItem.Id)); err != nil {
		return fmt.Errorf("failed to set show emby item id in manifest %d: %w", showFromAnime365.Anime365ID, err)
	}

	return nil
}

// UpdateShowMetadata updates Emby item with metadata from Anime 365.
// If item not found in Emby library error mediaserver.ErrItemNotFound returned.
// If entry not found in manifest error errShowNotFoundInManifest returned.
func (s *Service) UpdateShowMetadata(
	ctx context.Context,
	showFromAnime365 show.Show,
) error {
	embyItem, err := s.getSeriesItem(ctx, showFromAnime365.Anime365ID)
	if err != nil {
		if errors.Is(err, mediaserver.ErrItemNotFound) {
			return mediaserver.ErrItemNotFound
		} else if errors.Is(err, errShowNotFoundInManifest) {
			return errShowNotFoundInManifest
		} else {
			return fmt.Errorf(
				"failed to get series emby item for show %d: %w",
				showFromAnime365.Anime365ID,
				err,
			)
		}
	}

	needUpdate := false

	newName := showFromAnime365.TitleRomaji

	if showFromAnime365.TitleRussian != "" {
		newName = showFromAnime365.TitleRussian
	}

	if value(embyItem.Name) != newName {
		embyItem.Name = new(newName)
		needUpdate = true
	}

	if value(embyItem.SortName) != newName {
		embyItem.SortName = new(newName)
		needUpdate = true
	}

	if value(embyItem.OriginalTitle) != showFromAnime365.TitleRomaji {
		embyItem.OriginalTitle = new(showFromAnime365.TitleRomaji)
		needUpdate = true
	}

	if value(embyItem.CommunityRating) != float32(showFromAnime365.MyAnimeListScore) {
		embyItem.CommunityRating = new(float32(showFromAnime365.MyAnimeListScore))
		needUpdate = true
	}

	if value(embyItem.ProductionYear) != int32(showFromAnime365.Year) {
		embyItem.ProductionYear = new(int32(showFromAnime365.Year))
		needUpdate = true
	}

	newOverview := oneOrMoreLineBreaksRegexp.ReplaceAllString(showFromAnime365.Description, "\n\n")

	if value(embyItem.Overview) != newOverview {
		embyItem.Overview = new(newOverview)
		needUpdate = true
	}

	var (
		newTaglines []string
		newTags     []string
	)

	if showFromAnime365.TypeLabel != "" && showFromAnime365.SeasonLabel != "" {
		newTaglines = []string{fmt.Sprintf("%s, %s", showFromAnime365.TypeLabel, showFromAnime365.SeasonLabel)}
		newTags = []string{showFromAnime365.TypeLabel, showFromAnime365.SeasonLabel}
	} else if showFromAnime365.SeasonLabel != "" {
		newTaglines = []string{showFromAnime365.SeasonLabel}
		newTags = []string{showFromAnime365.SeasonLabel}
	} else if showFromAnime365.TypeLabel != "" {
		newTaglines = []string{showFromAnime365.TypeLabel}
		newTags = []string{showFromAnime365.TypeLabel}
	}

	if !slices.Equal(embyItem.Taglines, newTaglines) {
		embyItem.Taglines = newTaglines
		needUpdate = true
	}

	// Emby does not return Tags or TagItems fields in response.
	// So we always set new value to Tags field.
	// But it will be actually uploaded only if some other field updated too (if other field will trigger `needUpdate = true`).
	embyItem.Tags = newTags

	if !slices.Equal(embyItem.Genres, showFromAnime365.Genres) {
		embyItem.Genres = showFromAnime365.Genres
		needUpdate = true
	}

	newStatus := ""

	if len(showFromAnime365.EpisodePreviews) > 0 {
		if showFromAnime365.IsOngoing {
			newStatus = "Continuing"
		} else {
			newStatus = "Ended"
		}
	}

	if value(embyItem.Status) != newStatus {
		embyItem.Status = new(newStatus)
		needUpdate = true
	}

	if s.applyAnimeMappingExternalIDs(&embyItem, showFromAnime365.MyAnimeListID) {
		needUpdate = true
	}

	// Keep sidecar-owned textual metadata protected with field locks.
	embyItem.LockedFields = showLockedFields

	if needUpdate {
		if err := s.updateItem(ctx, embyItem); err != nil {
			return fmt.Errorf("failed to update show item %s: %w", value(embyItem.Id), err)
		}
	} else {
		s.logger.DebugContext(ctx, "No new metadata from Anime 365 found for Emby item, skipping updating it",
			slog.Int64("show_id", int64(showFromAnime365.Anime365ID)),
			slog.String("emby_item_id", value(embyItem.Id)),
		)
	}

	return nil
}

// UpdateShowMetadataFromShikimori updates Emby item with metadata from Shikimori.
// If item not found in Emby library error mediaserver.ErrItemNotFound returned.
// If entry not found in manifest error errShowNotFoundInManifest returned.
func (s *Service) UpdateShowMetadataFromShikimori(
	ctx context.Context,
	showID show.Anime365SeriesID,
	showFromShikimori show.ShowFromShikimori,
) error {
	embyItem, err := s.getSeriesItem(ctx, showID)
	if err != nil {
		if errors.Is(err, mediaserver.ErrItemNotFound) {
			return mediaserver.ErrItemNotFound
		} else if errors.Is(err, errShowNotFoundInManifest) {
			return errShowNotFoundInManifest
		} else {
			return fmt.Errorf(
				"failed to get series emby item for show %d: %w",
				showID,
				err,
			)
		}
	}

	needUpdate := false

	newPeople := make([]embyclient.BaseItemPerson, 0, len(showFromShikimori.StaffMembers))
	for _, staffMember := range showFromShikimori.StaffMembers {
		switch staffMember.Role {
		case show.StaffRoleDirector:
			newPeople = append(newPeople, embyclient.BaseItemPerson{
				Name: new(staffMember.Name),
				Type: new(embyclient.Director),
			})

		case show.StaffRoleProducer:
			newPeople = append(newPeople, embyclient.BaseItemPerson{
				Name: new(staffMember.Name),
				Type: new(embyclient.Producer),
			})

		case show.StaffRoleScript:
			newPeople = append(newPeople, embyclient.BaseItemPerson{
				Name: new(staffMember.Name),
				Type: new(embyclient.Writer),
			})

		case show.StaffRoleMusic:
			newPeople = append(newPeople, embyclient.BaseItemPerson{
				Name: new(staffMember.Name),
				Type: new(embyclient.Composer),
			})
		}
	}

	if !arePeopleEqual(embyItem.People, newPeople) {
		embyItem.People = newPeople
		needUpdate = true
	}

	newStudios := make([]embyclient.NameLongIdPair, 0, len(showFromShikimori.Studios))

	for _, studio := range showFromShikimori.Studios {
		newStudios = append(newStudios, embyclient.NameLongIdPair{
			Name: new(studio),
		})
	}

	if !areStringSliceAndNameLongIdPairSliceEqual(showFromShikimori.Studios, embyItem.Studios) {
		embyItem.Studios = newStudios
		needUpdate = true
	}

	if showFromShikimori.AverageEpisodeDuration != 0 {
		newRunTimeTicks := showFromShikimori.AverageEpisodeDuration.Nanoseconds() / 100

		if value(embyItem.RunTimeTicks) != newRunTimeTicks {
			embyItem.RunTimeTicks = new(showFromShikimori.AverageEpisodeDuration.Nanoseconds() / 100)
			needUpdate = true
		}
	}

	if value(embyItem.PremiereDate) != showFromShikimori.PremiereDate {
		embyItem.PremiereDate = new(showFromShikimori.PremiereDate)
		needUpdate = true
	}

	if value(embyItem.EndDate) != showFromShikimori.StoppedAiringAt {
		embyItem.EndDate = new(showFromShikimori.StoppedAiringAt)
		needUpdate = true
	}

	newOfficialRating := ""

	switch showFromShikimori.AgeRating {
	case show.AgeRatingEnumG:
		newOfficialRating = "G"
	case show.AgeRatingEnumPG:
		newOfficialRating = "PG"
	case show.AgeRatingEnumPG13:
		newOfficialRating = "PG-13"
	case show.AgeRatingEnumR:
		newOfficialRating = "R"
	case show.AgeRatingEnumRPlus:
		newOfficialRating = "RP"
	case show.AgeRatingEnumRx:
		newOfficialRating = "X"
	}

	if value(embyItem.OfficialRating) != newOfficialRating {
		embyItem.OfficialRating = new(newOfficialRating)
		needUpdate = true
	}

	// Keep sidecar-owned textual metadata protected with field locks.
	embyItem.LockedFields = showLockedFields

	if needUpdate {
		if err := s.updateItem(ctx, embyItem); err != nil {
			return fmt.Errorf("failed to update show item %s: %w", value(embyItem.Id), err)
		}
	} else {
		s.logger.DebugContext(ctx, "No new metadata from Anime 365 found for Emby item, skipping updating it",
			slog.Int64("show_id", int64(showID)),
			slog.String("emby_item_id", value(embyItem.Id)),
		)
	}

	return nil
}

// InitializeTranslationMetadata updates Emby item with metadata from Anime 365.
// If item not found in Emby library error mediaserver.ErrItemNotFound returned.
// If entry not found in manifest error errTranslationNotFoundInManifest returned.
func (s *Service) InitializeTranslationMetadata(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeEntity episode.Episode,
	translationEntity episode.Translation,
) error {
	embyItem, err := s.getEpisodeItem(ctx, showID, episodeEntity.Anime365ID, translationEntity.Anime365ID)
	if err != nil {
		if errors.Is(err, mediaserver.ErrItemNotFound) {
			return mediaserver.ErrItemNotFound
		} else if errors.Is(err, errTranslationNotFoundInManifest) {
			return errTranslationNotFoundInManifest
		} else {
			return fmt.Errorf(
				"failed to get episode emby item for translation %d: %w",
				translationEntity.Anime365ID,
				err,
			)
		}
	}

	embyItem.Name = new(episodeEntity.EpisodeLabel)
	embyItem.SortName = new(fmt.Sprintf(
		"Episode %05d, priority %010d",
		episodeEntity.EpisodeNumber,
		translationEntity.Anime365Priority,
	))

	embyItem.ForcedSortName = new(value(embyItem.SortName))

	if !translationEntity.MarkedAsActiveAt.IsZero() {
		embyItem.PremiereDate = new(translationEntity.MarkedAsActiveAt)
	} else if !episodeEntity.FirstUploadedAt.IsZero() {
		embyItem.PremiereDate = new(episodeEntity.FirstUploadedAt)
	}

	if episodeEntity.EpisodeNumber < math.MinInt32 || episodeEntity.EpisodeNumber > math.MaxInt32 {
		return errors.New("episode number is out of range int32")
	}

	embyItem.IndexNumber = new(int32(episodeEntity.EpisodeNumber))

	if _, err := s.applyEpisodeSeasonNumber(&embyItem, showID, episodeEntity); err != nil {
		return fmt.Errorf("failed to apply episode season number: %w", err)
	}

	embyItem.LockData = new(true)

	embyItem.LockedFields = []embyclient.MetadataFields{
		embyclient.Name,
		embyclient.SortName,
		embyclient.Tags,
		embyclient.Overview,
		embyclient.CommunityRating,
	}

	if err := s.updateItem(ctx, embyItem); err != nil {
		return fmt.Errorf("failed to update translation item %s: %w", value(embyItem.Id), err)
	}

	if err := s.manifestService.SetTranslationItemID(
		showID,
		episodeEntity.Anime365ID,
		translationEntity.Anime365ID,
		value(embyItem.Id),
	); err != nil {
		return fmt.Errorf(
			"failed to set translation emby item id in manifest %d: %w",
			translationEntity.Anime365ID,
			err,
		)
	}

	return nil
}

// UpdateTranslationMetadata updates Emby item with metadata from Anime 365.
// If item not found in Emby library error mediaserver.ErrItemNotFound returned.
// If entry not found in manifest error errTranslationNotFoundInManifest returned.
func (s *Service) UpdateTranslationMetadata(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeEntity episode.Episode,
	translationEntity episode.Translation,
) error {
	embyItem, err := s.getEpisodeItem(ctx, showID, episodeEntity.Anime365ID, translationEntity.Anime365ID)
	if err != nil {
		if errors.Is(err, mediaserver.ErrItemNotFound) {
			return mediaserver.ErrItemNotFound
		} else if errors.Is(err, errTranslationNotFoundInManifest) {
			return errTranslationNotFoundInManifest
		} else {
			return fmt.Errorf(
				"failed to get episode emby item for translation %d: %w",
				translationEntity.Anime365ID,
				err,
			)
		}
	}

	needUpdate, err := s.applyEpisodeSeasonNumber(&embyItem, showID, episodeEntity)
	if err != nil {
		return fmt.Errorf("failed to apply episode season number: %w", err)
	}

	newSortName := fmt.Sprintf(
		"Episode %05d, priority %010d",
		episodeEntity.EpisodeNumber,
		translationEntity.Anime365Priority,
	)

	if value(embyItem.SortName) != newSortName {
		embyItem.SortName = new(newSortName)
		needUpdate = true
	}

	if value(embyItem.ForcedSortName) != newSortName {
		embyItem.ForcedSortName = new(newSortName)
		needUpdate = true
	}

	if episodeEntity.EpisodeNumber < math.MinInt32 || episodeEntity.EpisodeNumber > math.MaxInt32 {
		return errors.New("episode number is out of range int32")
	}

	if value(embyItem.IndexNumber) != int32(episodeEntity.EpisodeNumber) {
		embyItem.IndexNumber = new(int32(episodeEntity.EpisodeNumber))
		needUpdate = true
	}

	embyItem.LockData = new(true)

	if needUpdate {
		if err := s.updateItem(ctx, embyItem); err != nil {
			return fmt.Errorf("failed to update translation item %s: %w", value(embyItem.Id), err)
		}
	} else {
		s.logger.DebugContext(ctx, "No new metadata from Anime 365 found for Emby item, skipping updating it",
			slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
			slog.Int64("translation_id", int64(translationEntity.Anime365ID)),
			slog.String("emby_item_id", value(embyItem.Id)),
		)
	}

	return nil
}

func (s *Service) GetLastWatchedEpisodeNumber(
	ctx context.Context,
	showID show.Anime365SeriesID,
) (int64, episode.Anime365TranslationID, error) {
	embyItem, err := s.getSeriesItem(ctx, showID)
	if err != nil {
		if errors.Is(err, mediaserver.ErrItemNotFound) {
			return 0, 0, mediaserver.ErrItemNotFound
		} else if errors.Is(err, errShowNotFoundInManifest) {
			return 0, 0, errShowNotFoundInManifest
		} else {
			return 0, 0, fmt.Errorf(
				"failed to get series emby item for show %d: %w",
				showID,
				err,
			)
		}
	}

	itemsResponse, err := getEmbyJSONResponse[embyclient.QueryResultBaseItemDto](
		s.embyClient.GetUsersByUseridItemsWithResponse(ctx, s.embyUserID, &embyclient.GetUsersByUseridItemsParams{
			Fields:           new(embyItemFields),
			ParentId:         new(value(embyItem.Id)),
			Limit:            new(int32(1)),
			IsPlayed:         new(true),
			SortBy:           new("SortName"),
			SortOrder:        new("Descending"),
			IncludeItemTypes: new("Episode"),
			Recursive:        new(true),
		}),
	)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get emby user items: %w", err)
	}

	if len(itemsResponse.Items) == 0 {
		return 0, 0, mediaserver.ErrItemNotFound
	}

	embyItem = itemsResponse.Items[0]

	videoFileRelativePath, err := filepath.Rel(s.embyLibraryRootDirectory, value(embyItem.Path))
	if err != nil {
		return 0, 0, fmt.Errorf(
			"failed to get episode video file path relative to emby library root directory: %w",
			err,
		)
	}

	_, _, translationID, exists := s.manifestService.GetIDsByVideoFileRelativePath(videoFileRelativePath)
	if !exists {
		return 0, 0, errors.New("failed to find translation in manifest by video file relative path")
	}

	return int64(value(embyItem.IndexNumber)), translationID, nil
}

func (s *Service) GetIDsWithoutMetadata(
	ctx context.Context,
) (map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{}, error) {
	itemsResponse, err := getEmbyJSONResponse[embyclient.QueryResultBaseItemDto](
		s.embyClient.GetItemsWithResponse(ctx, &embyclient.GetItemsParams{
			Fields:           new(embyItemFields),
			IncludeItemTypes: new("Episode"),
			ParentId:         new(s.embyLibraryItemID),
			Recursive:        new(true),
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get all emby episode items without metadata: %w", err)
	}

	result := make(map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{})

	for _, item := range itemsResponse.Items {
		videoFileRelativePath, err := filepath.Rel(s.embyLibraryRootDirectory, value(item.Path))
		if err != nil {
			s.logger.WarnContext(
				ctx,
				"Failed to get episode path relative to Emby library root directory",
				slog.Any("emby_item", item),
			)

			continue
		}

		showID, episodeID, translationID, exists := s.manifestService.GetIDsByVideoFileRelativePath(
			videoFileRelativePath,
		)
		if !exists {
			continue
		}

		_, hasItemID := s.manifestService.GetTranslationItemID(showID, episodeID, translationID)
		if hasItemID {
			continue // entry already has emby item id, so it has initial metadata
		}

		if _, ok := result[showID]; !ok {
			result[showID] = make(map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{})
		}

		if _, ok := result[showID][episodeID]; !ok {
			result[showID][episodeID] = make(map[episode.Anime365TranslationID]struct{})
		}

		result[showID][episodeID][translationID] = struct{}{}
	}

	return result, nil
}

func (s *Service) GetIDsWithMetadata(
	ctx context.Context,
) (map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{}, error) {
	itemsResponse, err := getEmbyJSONResponse[embyclient.QueryResultBaseItemDto](
		s.embyClient.GetItemsWithResponse(ctx, &embyclient.GetItemsParams{
			Fields:           new(embyItemFields),
			IncludeItemTypes: new("Episode"),
			ParentId:         new(s.embyLibraryItemID),
			Recursive:        new(true),
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get all emby episode items with metadata: %w", err)
	}

	result := make(map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{})

	for _, item := range itemsResponse.Items {
		videoFileRelativePath, err := filepath.Rel(s.embyLibraryRootDirectory, value(item.Path))
		if err != nil {
			s.logger.WarnContext(
				ctx,
				"Failed to get episode path relative to Emby library root directory",
				slog.Any("emby_item", item),
			)

			continue
		}

		showID, episodeID, translationID, exists := s.manifestService.GetIDsByVideoFileRelativePath(
			videoFileRelativePath,
		)
		if !exists {
			continue
		}

		_, hasItemID := s.manifestService.GetTranslationItemID(showID, episodeID, translationID)
		if !hasItemID {
			continue // entry already does not have emby item id, which means it did not receive initial metadata
		}

		if _, ok := result[showID]; !ok {
			result[showID] = make(map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{})
		}

		if _, ok := result[showID][episodeID]; !ok {
			result[showID][episodeID] = make(map[episode.Anime365TranslationID]struct{})
		}

		result[showID][episodeID][translationID] = struct{}{}
	}

	return result, nil
}

func (s *Service) GetEpisodeMetadataForNotification(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
) (mediaserver.EpisodeMetadataForNotification, error) {
	embyItem, err := s.getEpisodeItem(ctx, showID, episodeID, translationID)
	if err != nil {
		if errors.Is(err, mediaserver.ErrItemNotFound) {
			return mediaserver.EpisodeMetadataForNotification{}, mediaserver.ErrItemNotFound
		} else if errors.Is(err, errTranslationNotFoundInManifest) {
			return mediaserver.EpisodeMetadataForNotification{}, errTranslationNotFoundInManifest
		} else {
			return mediaserver.EpisodeMetadataForNotification{}, fmt.Errorf(
				"failed to get episode emby item for translation %d: %w",
				translationID,
				err,
			)
		}
	}

	q := url.Values{}
	q.Set("serverId", value(embyItem.ServerId))
	q.Set("id", value(embyItem.Id))

	webURL := *s.embyPublicURL
	webURL.Path = "/web/index.html"
	webURL.Fragment = "!/item?" + q.Encode()

	result := mediaserver.EpisodeMetadataForNotification{
		WebURL:       &webURL,
		SeriesName:   value(embyItem.SeriesName),
		EpisodeLabel: value(embyItem.Name),
	}

	for _, mediaSource := range embyItem.MediaSources {
		if mediaSource.Type != nil && *mediaSource.Type != embyclient.MediaSourceTypeDefault {
			continue
		}

		result.Bitrate = int64(value(mediaSource.Bitrate))

		for _, mediaStream := range mediaSource.MediaStreams {
			if mediaStream.Type != nil && *mediaStream.Type != embyclient.Video {
				continue
			}

			if !value(mediaStream.IsDefault) {
				continue
			}

			result.VideoMetadataDisplayTitle = value(mediaStream.DisplayTitle)
		}
	}

	if result.Bitrate == 0 || result.VideoMetadataDisplayTitle == "" {
		return mediaserver.EpisodeMetadataForNotification{}, fmt.Errorf(
			"failed to determine file bitrate and video metadata of emby item %s",
			value(embyItem.Id),
		)
	}

	return result, nil
}

// getEmbyShowPath returns show directory path as it should be in Emby library.
// If entry not found in manifest error errShowNotFoundInManifest returned.
func (s *Service) getEmbyShowPath(showID show.Anime365SeriesID) (string, error) {
	showDirectoryName, exists := s.manifestService.GetShowDirectoryName(showID)
	if !exists {
		return "", errShowNotFoundInManifest
	}

	return filepath.Join(s.embyLibraryRootDirectory, showDirectoryName), nil
}

// getEmbyTranslationPath returns translation video file path as it should be in Emby library.
// If entry not found in manifest error errTranslationNotFoundInManifest returned.
func (s *Service) getEmbyTranslationPath(
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
) (string, error) {
	translationRelativeFilePath, _, exists := s.manifestService.GetTranslationRelativePaths(
		showID,
		episodeID,
		translationID,
	)
	if !exists {
		return "", errTranslationNotFoundInManifest
	}

	return filepath.Join(s.embyLibraryRootDirectory, translationRelativeFilePath), nil
}

// getSeriesItem returns an Emby item with type "Series" by Anime 365 show ID.
// If item not found in Emby library error mediaserver.ErrItemNotFound returned.
// If entry not found in manifest error errShowNotFoundInManifest returned.
func (s *Service) getSeriesItem(
	ctx context.Context,
	showID show.Anime365SeriesID,
) (embyclient.BaseItemDto, error) {
	embySeriesPath, err := s.getEmbyShowPath(showID)
	if err != nil {
		if errors.Is(err, errShowNotFoundInManifest) {
			return embyclient.BaseItemDto{}, errShowNotFoundInManifest
		}

		return embyclient.BaseItemDto{}, fmt.Errorf(
			"failed to get show directory path for %d: %w",
			showID,
			err,
		)
	}

	itemsResponse, err := getEmbyJSONResponse[embyclient.QueryResultBaseItemDto](
		s.embyClient.GetItemsWithResponse(ctx, &embyclient.GetItemsParams{
			Fields:           new(embyItemFields),
			Path:             new(embySeriesPath),
			IncludeItemTypes: new("Series"),
			Recursive:        new(true),
			Limit:            new(int32(1)),
		}),
	)
	if err != nil {
		return embyclient.BaseItemDto{}, fmt.Errorf(
			"failed to get items from emby for show %d: %w",
			showID,
			err,
		)
	}

	if len(itemsResponse.Items) == 0 {
		return embyclient.BaseItemDto{}, mediaserver.ErrItemNotFound
	}

	return itemsResponse.Items[0], nil
}

// getEpisodeItem returns an Emby item with type "Episode" by Anime 365 IDs.
// If item not found in Emby library error mediaserver.ErrItemNotFound returned.
// If entry not found in manifest error errTranslationNotFoundInManifest returned.
func (s *Service) getEpisodeItem(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
) (embyclient.BaseItemDto, error) {
	embyEpisodeVideoPath, err := s.getEmbyTranslationPath(
		showID,
		episodeID,
		translationID,
	)
	if err != nil {
		if errors.Is(err, errTranslationNotFoundInManifest) {
			return embyclient.BaseItemDto{}, errTranslationNotFoundInManifest
		}

		return embyclient.BaseItemDto{}, fmt.Errorf(
			"failed to get translation video file path for %d: %w",
			translationID,
			err,
		)
	}

	itemsResponse, err := getEmbyJSONResponse[embyclient.QueryResultBaseItemDto](
		s.embyClient.GetItemsWithResponse(ctx, &embyclient.GetItemsParams{
			Fields:           new(embyItemFields),
			Path:             new(embyEpisodeVideoPath),
			IncludeItemTypes: new("Episode"),
			Recursive:        new(true),
			Limit:            new(int32(1)),
		}),
	)
	if err != nil {
		return embyclient.BaseItemDto{}, fmt.Errorf(
			"failed to get items from emby for translation %d: %w",
			translationID,
			err,
		)
	}

	if len(itemsResponse.Items) == 0 {
		return embyclient.BaseItemDto{}, mediaserver.ErrItemNotFound
	}

	return itemsResponse.Items[0], nil
}

func (s *Service) updateItem(ctx context.Context, embyItem embyclient.BaseItemDto) error {
	err := embyResponseError(s.embyClient.PostItemsByItemidWithResponse(ctx, value(embyItem.Id), embyItem))
	if err != nil {
		return fmt.Errorf("failed to update emby item %s via api: %w", value(embyItem.Id), err)
	}

	return nil
}

func areStringSliceAndNameLongIdPairSliceEqual(a []string, b []embyclient.NameLongIdPair) bool {
	if len(a) != len(b) {
		return false
	}

	counts := make(map[string]int, len(a))
	for _, s := range a {
		counts[s]++
	}

	for _, t := range b {
		if counts[value(t.Name)] == 0 {
			return false
		}

		counts[value(t.Name)]--
	}

	return true
}

func arePeopleEqual(a, b []embyclient.BaseItemPerson) bool {
	if len(a) != len(b) {
		return false
	}

	type key struct {
		Name string
		Type string
	}

	counts := make(map[key]int, len(a))
	// build frequency map for a
	for _, p := range a {
		t := ""
		if p.Type != nil {
			t = string(*p.Type)
		}

		k := key{
			Name: value(p.Name),
			Type: t,
		}
		counts[k]++
	}
	// subtract using b
	for _, p := range b {
		t := ""
		if p.Type != nil {
			t = string(*p.Type)
		}

		k := key{
			Name: value(p.Name),
			Type: t,
		}
		if counts[k] == 0 {
			return false
		}

		counts[k]--
	}

	return true
}

// embyItemFields includes fields read by the sidecar and fields required to preserve metadata when updating items.
const embyItemFields = "Budget,Chapters,CommunityRating,DateCreated,DisplayOrder,EndDate,EpisodeNumber,ForcedSortName,Genres,HomePageUrl,Id,IndexNumber,IndexOptions,LockData,LockedFields,MediaSources,MediaStreams,Name,OfficialRating,OriginalTitle,Overview,ParentId,ParentIndexNumber,Path,People,PremiereDate,PrimaryImageAspectRatio,ProductionYear,ProviderIds,Revenue,RunTimeTicks,SeriesName,ServerId,SortName,Status,Studios,TagItems,Taglines,Tags"

func value[T any](pointer *T) T {
	if pointer != nil {
		return *pointer
	}

	var zero T

	return zero
}

func embyResponseError(response interface {
	StatusCode() int
	Status() string
	GetBody() []byte
}, err error,
) error {
	if err != nil {
		return err
	}

	if response.StatusCode() < 200 || response.StatusCode() >= 300 {
		return fmt.Errorf("Emby API returned %s: %s", response.Status(), response.GetBody())
	}

	return nil
}

func getEmbyJSONResponse[T any](response interface {
	GetJSON200() *T
	StatusCode() int
	Status() string
	GetBody() []byte
}, err error,
) (T, error) {
	var zero T
	if err := embyResponseError(response, err); err != nil {
		return zero, err
	}

	if response.GetJSON200() == nil {
		return zero, errors.New("Emby API returned no JSON response")
	}

	return *response.GetJSON200(), nil
}
