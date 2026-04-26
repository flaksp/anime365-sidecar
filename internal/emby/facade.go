package emby

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/flaksp/anime365-sidecar/internal/emby/internal/manifest"
	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/authorslistformatter"
	"github.com/flaksp/anime365-sidecar/pkg/embyclient"
	"github.com/flaksp/anime365-sidecar/pkg/filename"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

var ErrEmbyItemNotFound = errors.New("emby item not found")

func NewService(
	downloadsDirectory string,
	embyUserID string,
	logger *slog.Logger,
	embyClient *embyclient.Client,
) *Service {
	return &Service{
		logger:             logger,
		manifestService:    manifest.NewService(downloadsDirectory, logger),
		embyClient:         embyClient,
		downloadsDirectory: downloadsDirectory,
		embyUserID:         embyUserID,
	}
}

type Service struct {
	logger                   *slog.Logger
	manifestService          *manifest.Service
	embyClient               *embyclient.Client
	downloadsDirectory       string
	embyLibraryItemID        string
	embyLibraryRootDirectory string
	embyUserID               string
}

func (s *Service) LoadManifestFromDisk(ctx context.Context) error {
	return s.manifestService.LoadFromDisk(ctx)
}

func (s *Service) DetectLibraryDirectoryFromEmby(ctx context.Context, libraryID string) error {
	virtualFolderDTOs, err := s.embyClient.GetLibraryVirtualFolders(ctx)
	if err != nil {
		return err
	}

	for _, virtualFolderDTO := range virtualFolderDTOs {
		if virtualFolderDTO.ItemId != libraryID {
			continue
		}

		if len(virtualFolderDTO.Locations) == 0 {
			return fmt.Errorf("expected library location list not empty for %s", virtualFolderDTO.Name)
		}

		s.embyLibraryItemID = virtualFolderDTO.ItemId
		s.embyLibraryRootDirectory = virtualFolderDTO.Locations[0]

		s.logger.InfoContext(
			ctx,
			"Detected library root metadata",
			slog.String("library_item_name", virtualFolderDTO.Name),
			slog.String("library_item_id", s.embyLibraryItemID),
			slog.String("library_root_directory", s.embyLibraryRootDirectory),
		)

		return nil
	}

	return fmt.Errorf("failed to detect library metadata, library with id \"%s\" does not exist", libraryID)
}

func (s *Service) CreateShowIfNotExists(
	showID show.Anime365SeriesID,
	showTitle string,
	myAnimeListID show.MyAnimeListID,
) error {
	showDirectoryName, showManifestEntryExists := s.manifestService.GetShowDirectoryName(showID)
	if !showManifestEntryExists {
		computedShowDirectoryName := filename.Clean(showTitle)
		computedShowDirectoryName = strings.Join(strings.Fields(computedShowDirectoryName), " ")
		computedShowDirectoryName = strings.TrimSpace(computedShowDirectoryName)
		computedShowDirectoryName = fmt.Sprintf("%s [anime365id=%d]", computedShowDirectoryName, showID)

		showDirectoryName = computedShowDirectoryName
	}

	showDirectoryAbsolutePath := filepath.Join(s.downloadsDirectory, showDirectoryName)

	err := s.createDirectoryIfNotExists(showDirectoryAbsolutePath)
	if err != nil {
		return fmt.Errorf("create show directory: %w", err)
	}

	if !showManifestEntryExists {
		err = s.manifestService.InsertShowEntry(showID, showDirectoryName, myAnimeListID)
		if err != nil {
			return fmt.Errorf("failed to set show entry: %w", err)
		}
	}

	return nil
}

func (s *Service) GetTranslationQuality(
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
) (int64, bool) {
	return s.manifestService.GetTranslationQuality(
		showID,
		episodeID,
		translationID,
	)
}

func (s *Service) DeleteTranslation(
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
) error {
	videoFileRelativePath, subtitlesFileRelativePath, exists := s.manifestService.GetTranslationRelativePaths(
		showID,
		episodeID,
		translationID,
	)
	if exists {
		videoFileAbsolutePath := filepath.Join(s.downloadsDirectory, videoFileRelativePath)

		err := s.deleteFileIfExists(videoFileAbsolutePath)
		if err != nil {
			return fmt.Errorf("delete video file: %w", err)
		}

		if subtitlesFileRelativePath != "" {
			subtitlesFileAbsolutePath := filepath.Join(s.downloadsDirectory, subtitlesFileRelativePath)

			err := s.deleteFileIfExists(subtitlesFileAbsolutePath)
			if err != nil {
				return fmt.Errorf("delete subtitle file: %w", err)
			}
		}
	}

	err := s.manifestService.DeleteTranslation(showID, episodeID, translationID)
	if err != nil {
		return fmt.Errorf("delete translation from manifest: %w", err)
	}

	return nil
}

func (s *Service) ComputeTranslationFileAbsolutePathsForDownloads(
	showEntity show.Show,
	episodeEntity episode.Episode,
	translationEntity episode.Translation,
	translationMedia episode.TranslationMedia,
) (string, string, error) {
	showDirectoryName, exists := s.manifestService.GetShowDirectoryName(showEntity.Anime365ID)
	if !exists {
		return "", "", errors.New("show does not exists in manifest")
	}

	var (
		translationFileName              string
		translationDirectoryRelativePath string
	)

	if episodeEntity.IsTrailer {
		translationDirectoryRelativePath = filepath.Join(showDirectoryName, "Trailers")
		translationDirectoryAbsolutePath := filepath.Join(s.downloadsDirectory, translationDirectoryRelativePath)

		err := s.createDirectoryIfNotExists(translationDirectoryAbsolutePath)
		if err != nil {
			return "", "", fmt.Errorf("failed to create trailers directory: %w", err)
		}

		translationFileName = fmt.Sprintf(
			"%s - %s %s by %s, ID %d",
			episodeEntity.EpisodeLabel,
			display.English.Languages().Name(translationEntity.Variant.Language),
			cases.Title(language.English).String(translationEntity.Variant.Kind.Label()),
			formatAuthorsListForFileName(translationEntity.Authors),
			translationEntity.Anime365ID,
		)
	} else if episodeEntity.IsSpecial {
		translationDirectoryRelativePath = filepath.Join(showDirectoryName, "Specials")
		translationDirectoryAbsolutePath := filepath.Join(s.downloadsDirectory, translationDirectoryRelativePath)

		err := s.createDirectoryIfNotExists(translationDirectoryAbsolutePath)
		if err != nil {
			return "", "", fmt.Errorf("failed to create specials directory: %w", err)
		}

		translationFileName = fmt.Sprintf(
			"%s - %s %s by %s, ID %d",
			episodeEntity.EpisodeLabel,
			display.English.Languages().Name(translationEntity.Variant.Language),
			cases.Title(language.English).String(translationEntity.Variant.Kind.Label()),
			formatAuthorsListForFileName(translationEntity.Authors),
			translationEntity.Anime365ID,
		)
	} else {
		translationFileName = fmt.Sprintf(
			"E%d - %s %s by %s, ID %d",
			episodeEntity.EpisodeNumber,
			display.English.Languages().Name(translationEntity.Variant.Language),
			cases.Title(language.English).String(translationEntity.Variant.Kind.Label()),
			formatAuthorsListForFileName(translationEntity.Authors),
			translationEntity.Anime365ID,
		)

		translationDirectoryRelativePath = showDirectoryName
	}

	translationFileName = filename.Clean(translationFileName)
	translationFileName = strings.Join(strings.Fields(translationFileName), " ")
	translationFileName = strings.TrimSpace(translationFileName)

	videoFileRelativePath := filepath.Join(translationDirectoryRelativePath, translationFileName+".mp4")
	videoFileAbsolutePath := filepath.Join(s.downloadsDirectory, videoFileRelativePath)

	var (
		subtitlesFileRelativePath string
		subtitlesFileAbsolutePath string
	)

	if translationMedia.SubtitlesURL != nil {
		subtitlesFileRelativePath = filepath.Join(translationDirectoryRelativePath, translationFileName+".ass")
		subtitlesFileAbsolutePath = filepath.Join(s.downloadsDirectory, subtitlesFileRelativePath)
	}

	return videoFileAbsolutePath, subtitlesFileAbsolutePath, nil
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
	videoFileRelativePath, err := filepath.Rel(s.downloadsDirectory, videoFileAbsolutePath)
	if err != nil {
		return fmt.Errorf("failed to get video file relative path: %w", err)
	}

	var subtitlesFileRelativePath string

	if subtitlesFileAbsolutePath != "" {
		subtitlesFileRelativePath, err = filepath.Rel(s.downloadsDirectory, subtitlesFileAbsolutePath)
		if err != nil {
			return fmt.Errorf("failed to get subtitles file relative path: %w", err)
		}
	}

	err = s.manifestService.SetTranslationEntry(
		showID,
		episodeID,
		translationID,
		videoFileRelativePath,
		subtitlesFileRelativePath,
		height,
	)
	if err != nil {
		return fmt.Errorf("failed to add translation entry to manifest: %w", err)
	}

	err = s.embyClient.RefreshLibrary(ctx)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to refresh library, but it's not critical")
	}

	return nil
}

var oneOrMoreLineBreaksRegexp = regexp.MustCompile(`\n+`)

// InitialUpdateShowMetadataWithAnime365Metadata updates Emby item with metadata from Anime 365.
// If item not found in Emby library error ErrEmbyItemNotFound returned.
// If entry not found in manifest error ErrShowNotFoundInManifest returned.
func (s *Service) InitialUpdateShowMetadataWithAnime365Metadata(
	ctx context.Context,
	showFromAnime365 show.Show,
) error {
	embyItem, err := s.getSeriesItem(ctx, showFromAnime365.Anime365ID)
	if err != nil {
		if errors.Is(err, ErrEmbyItemNotFound) {
			return ErrEmbyItemNotFound
		} else if errors.Is(err, ErrShowNotFoundInManifest) {
			return ErrShowNotFoundInManifest
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

	embyItem.Name = newName
	embyItem.SortName = newName

	embyItem.OriginalTitle = showFromAnime365.TitleRomaji

	if showFromAnime365.MyAnimeListScore > 0 {
		embyItem.CommunityRating = float32(showFromAnime365.MyAnimeListScore)
	}

	if showFromAnime365.Year > 0 {
		embyItem.ProductionYear = int32(showFromAnime365.Year)
	}

	if showFromAnime365.Description != "" {
		embyItem.Overview = oneOrMoreLineBreaksRegexp.ReplaceAllString(showFromAnime365.Description, "\n\n")
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
			embyItem.Status = "Continuing"
		} else {
			embyItem.Status = "Ended"
		}
	}

	embyItem.DisplayOrder = "absolute"

	embyItem.LockData = true

	embyItem.LockedFields = []embyclient.MetadataFields{
		embyclient.NAME_MetadataFields,
		embyclient.ORIGINAL_TITLE_MetadataFields,
		embyclient.COMMUNITY_RATING_MetadataFields,
		embyclient.OVERVIEW_MetadataFields,
		embyclient.GENRES_MetadataFields,
		embyclient.SORT_NAME_MetadataFields,
		embyclient.STUDIOS_MetadataFields,
		embyclient.OFFICIAL_RATING_MetadataFields,
		embyclient.CAST_MetadataFields,
		embyclient.TAGLINE_MetadataFields,
		embyclient.TAGS_MetadataFields,
	}

	err = s.embyClient.UpdateItem(ctx, embyItem.Id, embyItem)
	if err != nil {
		return fmt.Errorf("failed to update show item %s: %w", embyItem.Id, err)
	}

	if err = s.manifestService.SetShowEmbyItemID(showFromAnime365.Anime365ID, embyItem.Id); err != nil {
		return fmt.Errorf("failed to set show emby item id in manifest %d: %w", showFromAnime365.Anime365ID, err)
	}

	return nil
}

// UpdateShowMetadataWithAnime365Metadata updates Emby item with metadata from Anime 365.
// If item not found in Emby library error ErrEmbyItemNotFound returned.
// If entry not found in manifest error ErrShowNotFoundInManifest returned.
func (s *Service) UpdateShowMetadataWithAnime365Metadata(
	ctx context.Context,
	showFromAnime365 show.Show,
) error {
	embyItem, err := s.getSeriesItem(ctx, showFromAnime365.Anime365ID)
	if err != nil {
		if errors.Is(err, ErrEmbyItemNotFound) {
			return ErrEmbyItemNotFound
		} else if errors.Is(err, ErrShowNotFoundInManifest) {
			return ErrShowNotFoundInManifest
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

	if embyItem.Name != newName {
		embyItem.Name = newName
		needUpdate = true
	}

	if embyItem.SortName != newName {
		embyItem.SortName = newName
		needUpdate = true
	}

	if embyItem.OriginalTitle != showFromAnime365.TitleRomaji {
		embyItem.OriginalTitle = showFromAnime365.TitleRomaji
		needUpdate = true
	}

	if embyItem.CommunityRating != float32(showFromAnime365.MyAnimeListScore) {
		embyItem.CommunityRating = float32(showFromAnime365.MyAnimeListScore)
		needUpdate = true
	}

	if embyItem.ProductionYear != int32(showFromAnime365.Year) {
		embyItem.ProductionYear = int32(showFromAnime365.Year)
		needUpdate = true
	}

	newOverview := oneOrMoreLineBreaksRegexp.ReplaceAllString(showFromAnime365.Description, "\n\n")

	if embyItem.Overview != newOverview {
		embyItem.Overview = newOverview
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

	if embyItem.Status != newStatus {
		embyItem.Status = newStatus
		needUpdate = true
	}

	embyItem.LockData = true

	if needUpdate {
		err = s.embyClient.UpdateItem(ctx, embyItem.Id, embyItem)
		if err != nil {
			return fmt.Errorf("failed to update show item %s: %w", embyItem.Id, err)
		}
	} else {
		s.logger.DebugContext(ctx, "No new metadata from Anime 365 found for Emby item, skipping updating it",
			slog.Int64("show_id", int64(showFromAnime365.Anime365ID)),
			slog.String("emby_item_id", embyItem.Id),
		)
	}

	return nil
}

// UpdateShowMetadataWithShikimoriMetadata updates Emby item with metadata from Shikimori.
// If item not found in Emby library error ErrEmbyItemNotFound returned.
// If entry not found in manifest error ErrShowNotFoundInManifest returned.
func (s *Service) UpdateShowMetadataWithShikimoriMetadata(
	ctx context.Context,
	showID show.Anime365SeriesID,
	showFromShikimori show.ShowFromShikimori,
) error {
	embyItem, err := s.getSeriesItem(ctx, showID)
	if err != nil {
		if errors.Is(err, ErrEmbyItemNotFound) {
			return ErrEmbyItemNotFound
		} else if errors.Is(err, ErrShowNotFoundInManifest) {
			return ErrShowNotFoundInManifest
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
				Name:  staffMember.Name,
				Type_: new(embyclient.DIRECTOR_PersonType),
			})

		case show.StaffRoleProducer:
			newPeople = append(newPeople, embyclient.BaseItemPerson{
				Name:  staffMember.Name,
				Type_: new(embyclient.PRODUCER_PersonType),
			})

		case show.StaffRoleScript:
			newPeople = append(newPeople, embyclient.BaseItemPerson{
				Name:  staffMember.Name,
				Type_: new(embyclient.WRITER_PersonType),
			})

		case show.StaffRoleMusic:
			newPeople = append(newPeople, embyclient.BaseItemPerson{
				Name:  staffMember.Name,
				Type_: new(embyclient.COMPOSER_PersonType),
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
			Name: studio,
		})
	}

	if !areStringSliceAndNameLongIdPairSliceEqual(showFromShikimori.Studios, embyItem.Studios) {
		embyItem.Studios = newStudios
		needUpdate = true
	}

	if showFromShikimori.AverageEpisodeDuration != 0 {
		newRunTimeTicks := showFromShikimori.AverageEpisodeDuration.Nanoseconds() / 100

		if embyItem.RunTimeTicks != newRunTimeTicks {
			embyItem.RunTimeTicks = showFromShikimori.AverageEpisodeDuration.Nanoseconds() / 100
			needUpdate = true
		}
	}

	if embyItem.PremiereDate != showFromShikimori.PremiereDate {
		embyItem.PremiereDate = showFromShikimori.PremiereDate
		needUpdate = true
	}

	if embyItem.EndDate != showFromShikimori.StoppedAiringAt {
		embyItem.EndDate = showFromShikimori.StoppedAiringAt
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

	if embyItem.OfficialRating != newOfficialRating {
		embyItem.OfficialRating = newOfficialRating
		needUpdate = true
	}

	embyItem.LockData = true

	if needUpdate {
		err = s.embyClient.UpdateItem(ctx, embyItem.Id, embyItem)
		if err != nil {
			return fmt.Errorf("failed to update show item %s: %w", embyItem.Id, err)
		}
	} else {
		s.logger.DebugContext(ctx, "No new metadata from Anime 365 found for Emby item, skipping updating it",
			slog.Int64("show_id", int64(showID)),
			slog.String("emby_item_id", embyItem.Id),
		)
	}

	return nil
}

// InitialUpdateTranslationMetadataWithAnime365Metadata updates Emby item with metadata from Anime 365.
// If item not found in Emby library error ErrEmbyItemNotFound returned.
// If entry not found in manifest error ErrTranslationNotFoundInManifest returned.
func (s *Service) InitialUpdateTranslationMetadataWithAnime365Metadata(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeEntity episode.Episode,
	translationEntity episode.Translation,
) error {
	embyItem, err := s.getEpisodeItem(ctx, showID, episodeEntity.Anime365ID, translationEntity.Anime365ID)
	if err != nil {
		if errors.Is(err, ErrEmbyItemNotFound) {
			return ErrEmbyItemNotFound
		} else if errors.Is(err, ErrTranslationNotFoundInManifest) {
			return ErrTranslationNotFoundInManifest
		} else {
			return fmt.Errorf(
				"failed to get episode emby item for translation %d: %w",
				translationEntity.Anime365ID,
				err,
			)
		}
	}

	embyItem.Name = episodeEntity.EpisodeLabel
	embyItem.SortName = fmt.Sprintf(
		"Episode %05d, priority %010d",
		episodeEntity.EpisodeNumber,
		translationEntity.Anime365Priority,
	)

	embyItem.ForcedSortName = embyItem.SortName

	if !translationEntity.MarkedAsActiveAt.IsZero() {
		embyItem.PremiereDate = translationEntity.MarkedAsActiveAt
	} else if !episodeEntity.FirstUploadedAt.IsZero() {
		embyItem.PremiereDate = episodeEntity.FirstUploadedAt
	}

	if episodeEntity.EpisodeNumber < math.MinInt32 || episodeEntity.EpisodeNumber > math.MaxInt32 {
		return errors.New("episode number is out of range int32")
	}

	embyItem.IndexNumber = int32(episodeEntity.EpisodeNumber)

	embyItem.LockData = true

	embyItem.LockedFields = []embyclient.MetadataFields{
		embyclient.NAME_MetadataFields,
		embyclient.SORT_NAME_MetadataFields,
		embyclient.TAGS_MetadataFields,
		embyclient.OVERVIEW_MetadataFields,
		embyclient.COMMUNITY_RATING_MetadataFields,
	}

	err = s.embyClient.UpdateItem(ctx, embyItem.Id, embyItem)
	if err != nil {
		return fmt.Errorf("failed to update translation item %s: %w", embyItem.Id, err)
	}

	if err = s.manifestService.SetTranslationEmbyItemID(
		showID,
		episodeEntity.Anime365ID,
		translationEntity.Anime365ID,
		embyItem.Id,
	); err != nil {
		return fmt.Errorf(
			"failed to set translation emby item id in manifest %d: %w",
			translationEntity.Anime365ID,
			err,
		)
	}

	return nil
}

// UpdateTranslationMetadataWithAnime365Metadata updates Emby item with metadata from Anime 365.
// If item not found in Emby library error ErrEmbyItemNotFound returned.
// If entry not found in manifest error ErrTranslationNotFoundInManifest returned.
func (s *Service) UpdateTranslationMetadataWithAnime365Metadata(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeEntity episode.Episode,
	translationEntity episode.Translation,
) error {
	embyItem, err := s.getEpisodeItem(ctx, showID, episodeEntity.Anime365ID, translationEntity.Anime365ID)
	if err != nil {
		if errors.Is(err, ErrEmbyItemNotFound) {
			return ErrEmbyItemNotFound
		} else if errors.Is(err, ErrTranslationNotFoundInManifest) {
			return ErrTranslationNotFoundInManifest
		} else {
			return fmt.Errorf(
				"failed to get episode emby item for translation %d: %w",
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

	if embyItem.SortName != newSortName {
		embyItem.SortName = newSortName
		needUpdate = true
	}

	if embyItem.ForcedSortName != newSortName {
		embyItem.ForcedSortName = newSortName
		needUpdate = true
	}

	if episodeEntity.EpisodeNumber < math.MinInt32 || episodeEntity.EpisodeNumber > math.MaxInt32 {
		return errors.New("episode number is out of range int32")
	}

	if embyItem.IndexNumber != int32(episodeEntity.EpisodeNumber) {
		embyItem.IndexNumber = int32(episodeEntity.EpisodeNumber)
		needUpdate = true
	}

	embyItem.LockData = true

	if needUpdate {
		err = s.embyClient.UpdateItem(ctx, embyItem.Id, embyItem)
		if err != nil {
			return fmt.Errorf("failed to update translation item %s: %w", embyItem.Id, err)
		}
	} else {
		s.logger.DebugContext(ctx, "No new metadata from Anime 365 found for Emby item, skipping updating it",
			slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
			slog.Int64("translation_id", int64(translationEntity.Anime365ID)),
			slog.String("emby_item_id", embyItem.Id),
		)
	}

	return nil
}

// UpdateTranslationMetadataWithJikanMetadata updates Emby item with metadata from Jikan.
// If item not found in Emby library error ErrEmbyItemNotFound returned.
// If entry not found in manifest error ErrTranslationNotFoundInManifest returned.
func (s *Service) UpdateTranslationMetadataWithJikanMetadata(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
	episodeMetadataFromJikan episode.MetadataFromJikan,
) error {
	embyItem, err := s.getEpisodeItem(ctx, showID, episodeID, translationID)
	if err != nil {
		if errors.Is(err, ErrEmbyItemNotFound) {
			return ErrEmbyItemNotFound
		} else if errors.Is(err, ErrTranslationNotFoundInManifest) {
			return ErrTranslationNotFoundInManifest
		} else {
			return fmt.Errorf(
				"failed to get episode emby item for translation %d: %w",
				translationID,
				err,
			)
		}
	}

	needUpdate := false

	if embyItem.Name != episodeMetadataFromJikan.Title {
		embyItem.Name = episodeMetadataFromJikan.Title
		needUpdate = true
	}

	if !episodeMetadataFromJikan.AiredAt.IsZero() && embyItem.PremiereDate != episodeMetadataFromJikan.AiredAt {
		embyItem.PremiereDate = episodeMetadataFromJikan.AiredAt
		needUpdate = true

		year := episodeMetadataFromJikan.AiredAt.Year()

		if year >= math.MinInt32 && year <= math.MaxInt32 {
			embyItem.ProductionYear = int32(year)
		}
	}

	newOverview := oneOrMoreLineBreaksRegexp.ReplaceAllString(
		episodeMetadataFromJikan.Description,
		"\n\n",
	)

	if embyItem.Overview != newOverview {
		embyItem.Overview = newOverview
		needUpdate = true
	}

	if embyItem.CommunityRating != float32(episodeMetadataFromJikan.MyAnimeListScore) {
		embyItem.CommunityRating = float32(episodeMetadataFromJikan.MyAnimeListScore)
		needUpdate = true
	}

	newTags := make([]string, 0)

	if episodeMetadataFromJikan.IsFiller {
		newTags = append(newTags, "Филлер")
	}

	if episodeMetadataFromJikan.IsRecap {
		newTags = append(newTags, "Рекап")
	}

	if !areStringSliceAndNameLongIdPairSliceEqual(newTags, embyItem.TagItems) {
		embyItem.Tags = newTags
		needUpdate = true
	}

	embyItem.LockData = true

	if needUpdate {
		err = s.embyClient.UpdateItem(ctx, embyItem.Id, embyItem)
		if err != nil {
			return fmt.Errorf("failed to update translation item %s: %w", embyItem.Id, err)
		}
	} else {
		s.logger.DebugContext(ctx, "No new metadata from Jikan found for Emby item, skipping updating it",
			slog.Int64("translation_id", int64(translationID)),
			slog.String("emby_item_id", embyItem.Id),
		)
	}

	return nil
}

func (s *Service) GetIDs() map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{} {
	return s.manifestService.GetIDs()
}

func (s *Service) GetMyAnimeListIDToShowIDMap() map[show.MyAnimeListID]show.Anime365SeriesID {
	return s.manifestService.GetMyAnimeListIDToShowIDMap()
}

func (s *Service) IsPosterExists(showID show.Anime365SeriesID, posterURL *url.URL) (bool, error) {
	fileAbsolutePath, err := s.ComputePosterFileAbsolutePath(showID, posterURL)
	if err != nil {
		return false, fmt.Errorf("failed to compute poster file absolute path: %w", err)
	}

	fileExists, err := s.fileExists(fileAbsolutePath)
	if err != nil {
		return false, fmt.Errorf("failed to check if file exists: %w", err)
	}

	return fileExists, nil
}

func (s *Service) ComputePosterFileAbsolutePath(showID show.Anime365SeriesID, posterURL *url.URL) (string, error) {
	showDirectoryName, exists := s.manifestService.GetShowDirectoryName(showID)
	if !exists {
		return "", errors.New("show directory not found")
	}

	fileAbsolutePath := filepath.Join(s.downloadsDirectory, showDirectoryName, "poster"+filepath.Ext(posterURL.Path))

	return fileAbsolutePath, nil
}

func (s *Service) IsBackdropExists(
	showID show.Anime365SeriesID,
	screenshotID string,
) bool {
	return s.manifestService.IsBackdropExists(showID, screenshotID)
}

func (s *Service) ComputeBackdropFileAbsolutePath(
	showID show.Anime365SeriesID,
	imageURL *url.URL,
) (string, error) {
	showDirectoryName, exists := s.manifestService.GetShowDirectoryName(showID)
	if !exists {
		return "", errors.New("show directory not found")
	}

	backdropCount := s.manifestService.BackdropCount(showID)

	fileAbsolutePath := filepath.Join(
		s.downloadsDirectory,
		showDirectoryName,
		fmt.Sprintf("backdrop%d%s", backdropCount+1, filepath.Ext(imageURL.Path)),
	)

	return fileAbsolutePath, nil
}

func (s *Service) AddBackdrop(
	showID show.Anime365SeriesID,
	screenshotID string,
) error {
	err := s.manifestService.AddBackdrop(showID, screenshotID)
	if err != nil {
		return errors.New("failed to add backdrop to manifest")
	}

	return nil
}

func (s *Service) GetLastWatchedEpisodeNumber(
	ctx context.Context,
	showID show.Anime365SeriesID,
) (int64, episode.Anime365TranslationID, error) {
	embyShowPath, err := s.getEmbyShowPath(showID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get show path for %d: %w", showID, err)
	}

	itemsResponse, err := s.embyClient.GetItems(ctx, &embyclient.GetItemsOptionalParams{
		Path:             embyShowPath,
		IncludeItemTypes: []string{"Series"},
		Recursive:        new(true),
		Limit:            1,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get items from emby for show %d: %w", showID, err)
	}

	if len(itemsResponse.Items) == 0 {
		return 0, 0, ErrEmbyItemNotFound
	}

	showItem := itemsResponse.Items[0]

	itemsResponse, err = s.embyClient.GetUserItems(ctx, s.embyUserID, &embyclient.GetUserItemsOptionalParams{
		ParentID:         showItem.Id,
		Limit:            1,
		IsPlayed:         new(true),
		SortBy:           []string{"SortName"},
		SortOrder:        "Descending",
		IncludeItemTypes: []string{"Episode"},
		Recursive:        new(true),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get emby user items: %w", err)
	}

	if len(itemsResponse.Items) == 0 {
		return 0, 0, ErrEmbyItemNotFound
	}

	item := itemsResponse.Items[0]

	videoFileRelativePath, err := filepath.Rel(s.embyLibraryRootDirectory, item.Path)
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

	return int64(item.IndexNumber), translationID, nil
}

func (s *Service) GetIDsWithoutMetadataFromEmbyLibrary(
	ctx context.Context,
) (map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{}, error) {
	itemsResponse, err := s.embyClient.GetItems(ctx, &embyclient.GetItemsOptionalParams{
		IncludeItemTypes: []string{"Episode"},
		ParentID:         s.embyLibraryItemID,
		Recursive:        new(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get all emby episode items without metadata: %w", err)
	}

	result := make(map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{})

	for _, item := range itemsResponse.Items {
		videoFileRelativePath, err := filepath.Rel(s.embyLibraryRootDirectory, item.Path)
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

		_, hasEmbyItemID := s.manifestService.GetTranslationEmbyItemID(showID, episodeID, translationID)
		if hasEmbyItemID {
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

func (s *Service) GetIDsWithMetadataFromEmbyLibrary(
	ctx context.Context,
) (map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{}, error) {
	itemsResponse, err := s.embyClient.GetItems(ctx, &embyclient.GetItemsOptionalParams{
		IncludeItemTypes: []string{"Episode"},
		ParentID:         s.embyLibraryItemID,
		Recursive:        new(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get all emby episode items with metadata: %w", err)
	}

	result := make(map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{})

	for _, item := range itemsResponse.Items {
		videoFileRelativePath, err := filepath.Rel(s.embyLibraryRootDirectory, item.Path)
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

		_, hasEmbyItemID := s.manifestService.GetTranslationEmbyItemID(showID, episodeID, translationID)
		if !hasEmbyItemID {
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

func (s *Service) GetShowIDByDirectoryName(directoryName string) (show.Anime365SeriesID, bool) {
	return s.manifestService.GetShowIDByDirectoryName(directoryName)
}

func (s *Service) GetIDsByVideoFileRelativePath(
	videoFileRelativePath string,
) (show.Anime365SeriesID, episode.Anime365EpisodeID, episode.Anime365TranslationID, bool) {
	return s.manifestService.GetIDsByVideoFileRelativePath(videoFileRelativePath)
}

var (
	errNotDirectory = errors.New("not a directory")
	errNotFile      = errors.New("not a file")
)

func (s *Service) directoryExists(absolutePath string) (bool, error) {
	info, err := os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to check directory: %w", err)
	}

	if info.IsDir() {
		return true, nil
	}

	return false, errNotDirectory
}

func (s *Service) fileExists(absolutePath string) (bool, error) {
	info, err := os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to check file: %w", err)
	}

	if info.IsDir() {
		return false, errNotFile
	}

	return true, nil
}

func (s *Service) createDirectoryIfNotExists(absolutePath string) error {
	directoryExists, err := s.directoryExists(absolutePath)
	if err != nil {
		return fmt.Errorf("failed to check directory exists: %w", err)
	}

	if directoryExists {
		return nil
	}

	err = os.Mkdir(absolutePath, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

func (s *Service) deleteFileIfExists(absolutePath string) error {
	fileExists, err := s.fileExists(absolutePath)
	if err != nil {
		return fmt.Errorf("failed to check file exists: %w", err)
	}

	if !fileExists {
		return nil
	}

	err = os.Remove(absolutePath)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

var ErrShowNotFoundInManifest = errors.New("show not found in manifest")

// getEmbyShowPath returns show directory path as it should be in Emby library.
// If entry not found in manifest error ErrShowNotFoundInManifest returned.
func (s *Service) getEmbyShowPath(showID show.Anime365SeriesID) (string, error) {
	showDirectoryName, exists := s.manifestService.GetShowDirectoryName(showID)
	if !exists {
		return "", ErrShowNotFoundInManifest
	}

	return filepath.Join(s.embyLibraryRootDirectory, showDirectoryName), nil
}

var ErrTranslationNotFoundInManifest = errors.New("translation not found in manifest")

// getEmbyTranslationPath returns translation video file path as it should be in Emby library.
// If entry not found in manifest error ErrTranslationNotFoundInManifest returned.
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
		return "", ErrTranslationNotFoundInManifest
	}

	return filepath.Join(s.embyLibraryRootDirectory, translationRelativeFilePath), nil
}

// getSeriesItem returns an Emby item with type "Series" by Anime 365 show ID.
// If item not found in Emby library error ErrEmbyItemNotFound returned.
// If entry not found in manifest error ErrShowNotFoundInManifest returned.
func (s *Service) getSeriesItem(
	ctx context.Context,
	showID show.Anime365SeriesID,
) (embyclient.BaseItemDto, error) {
	embySeriesPath, err := s.getEmbyShowPath(showID)
	if err != nil {
		if errors.Is(err, ErrShowNotFoundInManifest) {
			return embyclient.BaseItemDto{}, ErrShowNotFoundInManifest
		}

		return embyclient.BaseItemDto{}, fmt.Errorf(
			"failed to get show directory path for %d: %w",
			showID,
			err,
		)
	}

	itemsResponse, err := s.embyClient.GetItems(
		ctx,
		&embyclient.GetItemsOptionalParams{
			Path:             embySeriesPath,
			IncludeItemTypes: []string{"Series"},
			Recursive:        new(true),
			Limit:            1,
		},
	)
	if err != nil {
		return embyclient.BaseItemDto{}, fmt.Errorf(
			"failed to get items from emby for show %d: %w",
			showID,
			err,
		)
	}

	if len(itemsResponse.Items) == 0 {
		return embyclient.BaseItemDto{}, ErrEmbyItemNotFound
	}

	return itemsResponse.Items[0], nil
}

// getEpisodeItem returns an Emby item with type "Episode" by Anime 365 IDs.
// If item not found in Emby library error ErrEmbyItemNotFound returned.
// If entry not found in manifest error ErrTranslationNotFoundInManifest returned.
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
		if errors.Is(err, ErrTranslationNotFoundInManifest) {
			return embyclient.BaseItemDto{}, ErrTranslationNotFoundInManifest
		}

		return embyclient.BaseItemDto{}, fmt.Errorf(
			"failed to get translation video file path for %d: %w",
			translationID,
			err,
		)
	}

	itemsResponse, err := s.embyClient.GetItems(
		ctx,
		&embyclient.GetItemsOptionalParams{
			Path:             embyEpisodeVideoPath,
			IncludeItemTypes: []string{"Episode"},
			Recursive:        new(true),
			Limit:            1,
		},
	)
	if err != nil {
		return embyclient.BaseItemDto{}, fmt.Errorf(
			"failed to get items from emby for translation %d: %w",
			translationID,
			err,
		)
	}

	if len(itemsResponse.Items) == 0 {
		return embyclient.BaseItemDto{}, ErrEmbyItemNotFound
	}

	return itemsResponse.Items[0], nil
}

func formatAuthorsListForFileName(authorsList []string) string {
	formattedAuthorsList := authorslistformatter.Format(authorsList)

	replacer := strings.NewReplacer("(", " ", ")", " ", " - ", " ")

	formattedAuthorsList = replacer.Replace(formattedAuthorsList)
	formattedAuthorsList = strings.Join(strings.Fields(formattedAuthorsList), " ")
	formattedAuthorsList = strings.TrimSpace(formattedAuthorsList)

	return formattedAuthorsList
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
		if counts[t.Name] == 0 {
			return false
		}

		counts[t.Name]--
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
		if p.Type_ != nil {
			t = string(*p.Type_)
		}

		k := key{
			Name: p.Name,
			Type: t,
		}
		counts[k]++
	}
	// subtract using b
	for _, p := range b {
		t := ""
		if p.Type_ != nil {
			t = string(*p.Type_)
		}

		k := key{
			Name: p.Name,
			Type: t,
		}
		if counts[k] == 0 {
			return false
		}

		counts[k]--
	}

	return true
}
