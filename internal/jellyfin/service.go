package jellyfin

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
	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/jellyfin/internal/manifest"
	"github.com/flaksp/anime365-sidecar/internal/libraryfiles"
	"github.com/flaksp/anime365-sidecar/internal/mediaserver"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/jellyfinclient"
	"github.com/google/uuid"
)

var _ mediaserver.Server = (*Service)(nil)

var (
	errShowNotFoundInManifest        = errors.New("show not found in manifest")
	errTranslationNotFoundInManifest = errors.New("translation not found in manifest")
)

func NewService(
	downloadsDirectory string,
	jellyfinUserID uuid.UUID,
	libraryID uuid.UUID,
	logger *slog.Logger,
	jellyfinClient *jellyfinclient.ClientWithResponses,
	jellyfinPublicURL *url.URL,
	animeMappingService *animemapping.Service,
) *Service {
	manifestService := manifest.NewService(downloadsDirectory, logger)

	return &Service{
		logger:              logger,
		manifestService:     manifestService,
		libraryID:           libraryID,
		Library:             libraryfiles.NewLibrary(downloadsDirectory, manifestService),
		jellyfinClient:      jellyfinClient,
		jellyfinUserID:      jellyfinUserID,
		jellyfinPublicURL:   jellyfinPublicURL,
		animeMappingService: animeMappingService,
	}
}

type Service struct {
	manifestService *manifest.Service
	logger          *slog.Logger
	*libraryfiles.Library
	jellyfinClient               *jellyfinclient.ClientWithResponses
	jellyfinPublicURL            *url.URL
	animeMappingService          *animemapping.Service
	jellyfinLibraryRootDirectory string
	libraryID                    uuid.UUID
	jellyfinLibraryItemID        uuid.UUID
	jellyfinUserID               uuid.UUID
}

// Initialize loads adapter-owned state before discovering the configured library.
func (s *Service) Initialize(ctx context.Context) error {
	if err := s.manifestService.LoadFromDisk(ctx); err != nil {
		return err
	}

	return s.detectLibraryDirectory(ctx, s.libraryID)
}

func (s *Service) RefreshLibrary(ctx context.Context) error {
	return jellyfinResponseError(s.jellyfinClient.RefreshLibraryWithResponse(ctx))
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

var showLockedFields = []jellyfinclient.MetadataField{
	jellyfinclient.MetadataFieldName,
	jellyfinclient.MetadataFieldOverview,
	jellyfinclient.MetadataFieldGenres,
	jellyfinclient.MetadataFieldStudios,
	jellyfinclient.MetadataFieldRuntime,
	jellyfinclient.MetadataFieldOfficialRating,
	jellyfinclient.MetadataFieldCast,
	jellyfinclient.MetadataFieldTags,
}

// InitializeShowMetadata updates Jellyfin item with metadata from Anime 365.
// If item not found in Jellyfin library error mediaserver.ErrItemNotFound returned.
// If entry not found in manifest error errShowNotFoundInManifest returned.
func (s *Service) InitializeShowMetadata(
	ctx context.Context,
	showFromAnime365 show.Show,
) error {
	jellyfinItem, err := s.getSeriesItem(ctx, showFromAnime365.Anime365ID)
	if err != nil {
		if errors.Is(err, mediaserver.ErrItemNotFound) {
			return mediaserver.ErrItemNotFound
		} else if errors.Is(err, errShowNotFoundInManifest) {
			return errShowNotFoundInManifest
		} else {
			return fmt.Errorf(
				"failed to get series jellyfin item for show %d: %w",
				showFromAnime365.Anime365ID,
				err,
			)
		}
	}

	newName := showFromAnime365.TitleRomaji

	if showFromAnime365.TitleRussian != "" {
		newName = showFromAnime365.TitleRussian
	}

	jellyfinItem.Name = new(newName)
	jellyfinItem.SortName = new(newName)

	jellyfinItem.OriginalTitle = new(showFromAnime365.TitleRomaji)

	if showFromAnime365.MyAnimeListScore > 0 {
		jellyfinItem.CommunityRating = new(float32(showFromAnime365.MyAnimeListScore))
	}

	if showFromAnime365.Year > 0 {
		jellyfinItem.ProductionYear = new(int32(showFromAnime365.Year))
	}

	if showFromAnime365.Description != "" {
		jellyfinItem.Overview = new(oneOrMoreLineBreaksRegexp.ReplaceAllString(showFromAnime365.Description, "\n\n"))
	}

	if showFromAnime365.TypeLabel != "" && showFromAnime365.SeasonLabel != "" {
		jellyfinItem.Taglines = []string{
			fmt.Sprintf("%s, %s", showFromAnime365.TypeLabel, showFromAnime365.SeasonLabel),
		}
		jellyfinItem.Tags = []string{showFromAnime365.TypeLabel, showFromAnime365.SeasonLabel}
	} else if showFromAnime365.SeasonLabel != "" {
		jellyfinItem.Taglines = []string{showFromAnime365.SeasonLabel}
		jellyfinItem.Tags = []string{showFromAnime365.SeasonLabel}
	} else if showFromAnime365.TypeLabel != "" {
		jellyfinItem.Taglines = []string{showFromAnime365.TypeLabel}
		jellyfinItem.Tags = []string{showFromAnime365.TypeLabel}
	}

	if len(showFromAnime365.Genres) > 0 {
		jellyfinItem.Genres = showFromAnime365.Genres
	}

	if len(showFromAnime365.EpisodePreviews) > 0 {
		if showFromAnime365.IsOngoing {
			jellyfinItem.Status = new("Continuing")
		} else {
			jellyfinItem.Status = new("Ended")
		}
	}

	s.applyAnimeMappingExternalIDs(&jellyfinItem, showFromAnime365.MyAnimeListID)

	jellyfinItem.DisplayOrder = new("absolute")

	// Keep sidecar-owned textual metadata protected with field locks.
	jellyfinItem.LockedFields = showLockedFields
	jellyfinItem.LockData = new(true)

	if err := s.updateItem(ctx, jellyfinItem); err != nil {
		return fmt.Errorf("failed to update show item %s: %w", value(jellyfinItem.Id).String(), err)
	}

	if err := jellyfinResponseError(
		s.jellyfinClient.RefreshItemWithResponse(ctx, value(jellyfinItem.Id), &jellyfinclient.RefreshItemParams{
			MetadataRefreshMode: new(jellyfinclient.MetadataRefreshModeFullRefresh),
			ImageRefreshMode:    new(jellyfinclient.MetadataRefreshModeFullRefresh),
			ReplaceAllMetadata:  new(false),
			ReplaceAllImages:    new(false),
		}),
	); err != nil {
		return fmt.Errorf("refresh show images: %w", err)
	}

	if err := s.manifestService.SetShowItemID(
		showFromAnime365.Anime365ID,
		value(jellyfinItem.Id).String(),
	); err != nil {
		return fmt.Errorf("failed to set show jellyfin item id in manifest %d: %w", showFromAnime365.Anime365ID, err)
	}

	return nil
}

// UpdateShowMetadata updates Jellyfin item with metadata from Anime 365.
// If item not found in Jellyfin library error mediaserver.ErrItemNotFound returned.
// If entry not found in manifest error errShowNotFoundInManifest returned.
func (s *Service) UpdateShowMetadata(
	ctx context.Context,
	showFromAnime365 show.Show,
) error {
	jellyfinItem, err := s.getSeriesItem(ctx, showFromAnime365.Anime365ID)
	if err != nil {
		if errors.Is(err, mediaserver.ErrItemNotFound) {
			return mediaserver.ErrItemNotFound
		} else if errors.Is(err, errShowNotFoundInManifest) {
			return errShowNotFoundInManifest
		} else {
			return fmt.Errorf(
				"failed to get series jellyfin item for show %d: %w",
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

	if value(jellyfinItem.Name) != newName {
		jellyfinItem.Name = new(newName)
		needUpdate = true
	}

	if value(jellyfinItem.SortName) != newName {
		jellyfinItem.SortName = new(newName)
		needUpdate = true
	}

	if value(jellyfinItem.OriginalTitle) != showFromAnime365.TitleRomaji {
		jellyfinItem.OriginalTitle = new(showFromAnime365.TitleRomaji)
		needUpdate = true
	}

	if value(jellyfinItem.CommunityRating) != float32(showFromAnime365.MyAnimeListScore) {
		jellyfinItem.CommunityRating = new(float32(showFromAnime365.MyAnimeListScore))
		needUpdate = true
	}

	if value(jellyfinItem.ProductionYear) != int32(showFromAnime365.Year) {
		jellyfinItem.ProductionYear = new(int32(showFromAnime365.Year))
		needUpdate = true
	}

	newOverview := oneOrMoreLineBreaksRegexp.ReplaceAllString(showFromAnime365.Description, "\n\n")

	if value(jellyfinItem.Overview) != newOverview {
		jellyfinItem.Overview = new(newOverview)
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

	if !slices.Equal(jellyfinItem.Taglines, newTaglines) {
		jellyfinItem.Taglines = newTaglines
		needUpdate = true
	}

	if !slices.Equal(jellyfinItem.Tags, newTags) {
		jellyfinItem.Tags = newTags
		needUpdate = true
	}

	if !slices.Equal(jellyfinItem.Genres, showFromAnime365.Genres) {
		jellyfinItem.Genres = showFromAnime365.Genres
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

	if value(jellyfinItem.Status) != newStatus {
		jellyfinItem.Status = new(newStatus)
		needUpdate = true
	}

	if s.applyAnimeMappingExternalIDs(&jellyfinItem, showFromAnime365.MyAnimeListID) {
		needUpdate = true
	}

	// Keep sidecar-owned textual metadata protected with field locks.
	jellyfinItem.LockedFields = showLockedFields
	jellyfinItem.LockData = new(true)

	if needUpdate {
		if err := s.updateItem(ctx, jellyfinItem); err != nil {
			return fmt.Errorf("failed to update show item %s: %w", value(jellyfinItem.Id).String(), err)
		}
	} else {
		s.logger.DebugContext(ctx, "No new metadata from Anime 365 found for Jellyfin item, skipping updating it",
			slog.Int64("show_id", int64(showFromAnime365.Anime365ID)),
			slog.String("jellyfin_item_id", value(jellyfinItem.Id).String()),
		)
	}

	return nil
}

// UpdateShowMetadataFromShikimori updates Jellyfin item with metadata from Shikimori.
// If item not found in Jellyfin library error mediaserver.ErrItemNotFound returned.
// If entry not found in manifest error errShowNotFoundInManifest returned.
func (s *Service) UpdateShowMetadataFromShikimori(
	ctx context.Context,
	showID show.Anime365SeriesID,
	showFromShikimori show.ShowFromShikimori,
) error {
	jellyfinItem, err := s.getSeriesItem(ctx, showID)
	if err != nil {
		if errors.Is(err, mediaserver.ErrItemNotFound) {
			return mediaserver.ErrItemNotFound
		} else if errors.Is(err, errShowNotFoundInManifest) {
			return errShowNotFoundInManifest
		} else {
			return fmt.Errorf(
				"failed to get series jellyfin item for show %d: %w",
				showID,
				err,
			)
		}
	}

	needUpdate := false

	newPeople := make([]jellyfinclient.BaseItemPerson, 0, len(showFromShikimori.StaffMembers))
	for _, staffMember := range showFromShikimori.StaffMembers {
		switch staffMember.Role {
		case show.StaffRoleDirector:
			newPeople = append(newPeople, jellyfinclient.BaseItemPerson{
				Name: new(staffMember.Name),
				Type: new(jellyfinclient.PersonKindDirector),
			})

		case show.StaffRoleProducer:
			newPeople = append(newPeople, jellyfinclient.BaseItemPerson{
				Name: new(staffMember.Name),
				Type: new(jellyfinclient.PersonKindProducer),
			})

		case show.StaffRoleScript:
			newPeople = append(newPeople, jellyfinclient.BaseItemPerson{
				Name: new(staffMember.Name),
				Type: new(jellyfinclient.PersonKindWriter),
			})

		case show.StaffRoleMusic:
			newPeople = append(newPeople, jellyfinclient.BaseItemPerson{
				Name: new(staffMember.Name),
				Type: new(jellyfinclient.PersonKindComposer),
			})
		}
	}

	if !arePeopleEqual(jellyfinItem.People, newPeople) {
		jellyfinItem.People = newPeople
		needUpdate = true
	}

	newStudios := make([]jellyfinclient.NameGuidPair, 0, len(showFromShikimori.Studios))

	for _, studio := range showFromShikimori.Studios {
		newStudios = append(newStudios, jellyfinclient.NameGuidPair{
			Name: new(studio),
		})
	}

	if !areStringSliceAndNameGuidPairSliceEqual(showFromShikimori.Studios, jellyfinItem.Studios) {
		jellyfinItem.Studios = newStudios
		needUpdate = true
	}

	if showFromShikimori.AverageEpisodeDuration != 0 {
		newRunTimeTicks := showFromShikimori.AverageEpisodeDuration.Nanoseconds() / 100

		if value(jellyfinItem.RunTimeTicks) != newRunTimeTicks {
			jellyfinItem.RunTimeTicks = new(showFromShikimori.AverageEpisodeDuration.Nanoseconds() / 100)
			needUpdate = true
		}
	}

	if value(jellyfinItem.PremiereDate) != showFromShikimori.PremiereDate {
		jellyfinItem.PremiereDate = new(showFromShikimori.PremiereDate)
		needUpdate = true
	}

	if value(jellyfinItem.EndDate) != showFromShikimori.StoppedAiringAt {
		jellyfinItem.EndDate = new(showFromShikimori.StoppedAiringAt)
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

	if value(jellyfinItem.OfficialRating) != newOfficialRating {
		jellyfinItem.OfficialRating = new(newOfficialRating)
		needUpdate = true
	}

	// Keep sidecar-owned textual metadata protected with field locks.
	jellyfinItem.LockedFields = showLockedFields
	jellyfinItem.LockData = new(true)

	if needUpdate {
		if err := s.updateItem(ctx, jellyfinItem); err != nil {
			return fmt.Errorf("failed to update show item %s: %w", value(jellyfinItem.Id).String(), err)
		}
	} else {
		s.logger.DebugContext(ctx, "No new metadata from Anime 365 found for Jellyfin item, skipping updating it",
			slog.Int64("show_id", int64(showID)),
			slog.String("jellyfin_item_id", value(jellyfinItem.Id).String()),
		)
	}

	return nil
}

// InitializeTranslationMetadata updates Jellyfin item with metadata from Anime 365.
// If item not found in Jellyfin library error mediaserver.ErrItemNotFound returned.
// If entry not found in manifest error errTranslationNotFoundInManifest returned.
func (s *Service) InitializeTranslationMetadata(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeEntity episode.Episode,
	translationEntity episode.Translation,
) error {
	jellyfinItem, err := s.getEpisodeItem(ctx, showID, episodeEntity.Anime365ID, translationEntity.Anime365ID)
	if err != nil {
		if errors.Is(err, mediaserver.ErrItemNotFound) {
			return mediaserver.ErrItemNotFound
		} else if errors.Is(err, errTranslationNotFoundInManifest) {
			return errTranslationNotFoundInManifest
		} else {
			return fmt.Errorf(
				"failed to get episode jellyfin item for translation %d: %w",
				translationEntity.Anime365ID,
				err,
			)
		}
	}

	jellyfinItem.Name = new(episodeEntity.EpisodeLabel)
	jellyfinItem.SortName = new(fmt.Sprintf(
		"Episode %05d, priority %010d",
		episodeEntity.EpisodeNumber,
		translationEntity.Anime365Priority,
	))

	jellyfinItem.ForcedSortName = new(value(jellyfinItem.SortName))

	if !translationEntity.MarkedAsActiveAt.IsZero() {
		jellyfinItem.PremiereDate = new(translationEntity.MarkedAsActiveAt)
	} else if !episodeEntity.FirstUploadedAt.IsZero() {
		jellyfinItem.PremiereDate = new(episodeEntity.FirstUploadedAt)
	}

	if episodeEntity.EpisodeNumber < math.MinInt32 || episodeEntity.EpisodeNumber > math.MaxInt32 {
		return errors.New("episode number is out of range int32")
	}

	jellyfinItem.IndexNumber = new(int32(episodeEntity.EpisodeNumber))

	jellyfinItem.LockData = new(true)

	jellyfinItem.LockedFields = []jellyfinclient.MetadataField{
		jellyfinclient.MetadataFieldName,
		jellyfinclient.MetadataFieldTags,
		jellyfinclient.MetadataFieldOverview,
	}

	if err := s.updateItem(ctx, jellyfinItem); err != nil {
		return fmt.Errorf("failed to update translation item %s: %w", value(jellyfinItem.Id).String(), err)
	}

	if err := s.manifestService.SetTranslationItemID(
		showID,
		episodeEntity.Anime365ID,
		translationEntity.Anime365ID,
		value(jellyfinItem.Id).String(),
	); err != nil {
		return fmt.Errorf(
			"failed to set translation jellyfin item id in manifest %d: %w",
			translationEntity.Anime365ID,
			err,
		)
	}

	return nil
}

// UpdateTranslationMetadata updates Jellyfin item with metadata from Anime 365.
// If item not found in Jellyfin library error mediaserver.ErrItemNotFound returned.
// If entry not found in manifest error errTranslationNotFoundInManifest returned.
func (s *Service) UpdateTranslationMetadata(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeEntity episode.Episode,
	translationEntity episode.Translation,
) error {
	jellyfinItem, err := s.getEpisodeItem(ctx, showID, episodeEntity.Anime365ID, translationEntity.Anime365ID)
	if err != nil {
		if errors.Is(err, mediaserver.ErrItemNotFound) {
			return mediaserver.ErrItemNotFound
		} else if errors.Is(err, errTranslationNotFoundInManifest) {
			return errTranslationNotFoundInManifest
		} else {
			return fmt.Errorf(
				"failed to get episode jellyfin item for translation %d: %w",
				translationEntity.Anime365ID,
				err,
			)
		}
	}

	needUpdate := false

	newSortName := fmt.Sprintf(
		"Episode %05d, priority %010d",
		episodeEntity.EpisodeNumber,
		translationEntity.Anime365Priority,
	)

	if value(jellyfinItem.SortName) != newSortName {
		jellyfinItem.SortName = new(newSortName)
		needUpdate = true
	}

	if value(jellyfinItem.ForcedSortName) != newSortName {
		jellyfinItem.ForcedSortName = new(newSortName)
		needUpdate = true
	}

	if episodeEntity.EpisodeNumber < math.MinInt32 || episodeEntity.EpisodeNumber > math.MaxInt32 {
		return errors.New("episode number is out of range int32")
	}

	if value(jellyfinItem.IndexNumber) != int32(episodeEntity.EpisodeNumber) {
		jellyfinItem.IndexNumber = new(int32(episodeEntity.EpisodeNumber))
		needUpdate = true
	}

	jellyfinItem.LockData = new(true)

	if needUpdate {
		if err := s.updateItem(ctx, jellyfinItem); err != nil {
			return fmt.Errorf("failed to update translation item %s: %w", value(jellyfinItem.Id).String(), err)
		}
	} else {
		s.logger.DebugContext(ctx, "No new metadata from Anime 365 found for Jellyfin item, skipping updating it",
			slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
			slog.Int64("translation_id", int64(translationEntity.Anime365ID)),
			slog.String("jellyfin_item_id", value(jellyfinItem.Id).String()),
		)
	}

	return nil
}

// getJellyfinShowPath returns show directory path as it should be in Jellyfin library.
// If entry not found in manifest error errShowNotFoundInManifest returned.
func (s *Service) getJellyfinShowPath(showID show.Anime365SeriesID) (string, error) {
	showDirectoryName, exists := s.manifestService.GetShowDirectoryName(showID)
	if !exists {
		return "", errShowNotFoundInManifest
	}

	return filepath.Join(s.jellyfinLibraryRootDirectory, showDirectoryName), nil
}

// getJellyfinTranslationPath returns translation video file path as it should be in Jellyfin library.
// If entry not found in manifest error errTranslationNotFoundInManifest returned.
func (s *Service) getJellyfinTranslationPath(
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

	return filepath.Join(s.jellyfinLibraryRootDirectory, translationRelativeFilePath), nil
}

func (s *Service) updateItem(ctx context.Context, jellyfinItem jellyfinclient.BaseItemDto) error {
	err := jellyfinResponseError(s.jellyfinClient.UpdateItemWithResponse(ctx, value(jellyfinItem.Id), jellyfinItem))
	if err != nil {
		return fmt.Errorf("failed to update jellyfin item %s via api: %w", value(jellyfinItem.Id).String(), err)
	}

	return nil
}

func areStringSliceAndNameGuidPairSliceEqual(a []string, b []jellyfinclient.NameGuidPair) bool {
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

func arePeopleEqual(a, b []jellyfinclient.BaseItemPerson) bool {
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

func value[T any](pointer *T) T {
	if pointer != nil {
		return *pointer
	}

	var zero T

	return zero
}

func jellyfinResponseError(response interface {
	StatusCode() int
	Status() string
	GetBody() []byte
}, err error,
) error {
	if err != nil {
		return err
	}

	if response.StatusCode() < 200 || response.StatusCode() >= 300 {
		return fmt.Errorf("Jellyfin API returned %s: %s", response.Status(), response.GetBody())
	}

	return nil
}

func getJellyfinJSONResponse[T any](response interface {
	GetJSON200() *T
	StatusCode() int
	Status() string
	GetBody() []byte
}, err error,
) (T, error) {
	var zero T
	if err := jellyfinResponseError(response, err); err != nil {
		return zero, err
	}

	if response.GetJSON200() == nil {
		return zero, errors.New("Jellyfin API returned no JSON response")
	}

	return *response.GetJSON200(), nil
}
