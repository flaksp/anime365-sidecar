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
	"strconv"
	"strings"

	"github.com/flaksp/anime365-sidecar/internal/emby/internal/manifest"
	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/show"
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

func (s *Service) DetectLibraryDirectoryFromEmby(ctx context.Context, expectedLibraryName string) error {
	virtualFolderDTOs, err := s.embyClient.GetLibraryVirtualFolders(ctx)
	if err != nil {
		return err
	}

	for _, virtualFolderDTO := range virtualFolderDTOs {
		if virtualFolderDTO.Name != expectedLibraryName {
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

	return fmt.Errorf("failed to detect library metadata, library with name \"%s\" does not exist", expectedLibraryName)
}

func (s *Service) CreateShowIfNotExists(
	showID show.Anime365SeriesID,
	showTitle string,
	myAnimeListID show.MyAnimeListID,
) error {
	showDirectoryName, showManifestEntryExists := s.manifestService.GetShowDirectoryName(showID)
	if !showManifestEntryExists {
		computedShowDirectoryName := filename.Clean(showTitle)
		computedShowDirectoryName = strings.TrimSpace(computedShowDirectoryName)
		computedShowDirectoryName = strings.Join(strings.Fields(computedShowDirectoryName), " ")
		computedShowDirectoryName = fmt.Sprintf("%s [anime365id=%d]", computedShowDirectoryName, showID)

		showDirectoryName = computedShowDirectoryName
	}

	showDirectoryAbsolutePath := filepath.Join(s.downloadsDirectory, showDirectoryName)

	err := s.createDirectoryIfNotExists(showDirectoryAbsolutePath)
	if err != nil {
		return fmt.Errorf("create show directory: %w", err)
	}

	err = s.manifestService.SetShowEntry(showID, showDirectoryName, myAnimeListID)
	if err != nil {
		return fmt.Errorf("failed to set show entry: %w", err)
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
		translationDirectoryAbsolutePath string
	)

	if episodeEntity.IsTrailer {
		translationDirectoryRelativePath = filepath.Join(showDirectoryName, "Trailers")
		translationDirectoryAbsolutePath = filepath.Join(s.downloadsDirectory, translationDirectoryRelativePath)

		err := s.createDirectoryIfNotExists(translationDirectoryAbsolutePath)
		if err != nil {
			return "", "", fmt.Errorf("failed to create trailers directory: %w", err)
		}

		translationFileName = fmt.Sprintf(
			"%s - %s %s by %s",
			episodeEntity.EpisodeLabel,
			display.English.Languages().Name(translationEntity.Variant.Language),
			cases.Title(language.English).String(translationEntity.Variant.Kind.Label()),
			formatAuthorsList(translationEntity.Authors),
		)
	} else if episodeEntity.IsSpecial {
		translationDirectoryRelativePath = filepath.Join(showDirectoryName, "Specials")
		translationDirectoryAbsolutePath = filepath.Join(s.downloadsDirectory, translationDirectoryRelativePath)

		err := s.createDirectoryIfNotExists(translationDirectoryAbsolutePath)
		if err != nil {
			return "", "", fmt.Errorf("failed to create specials directory: %w", err)
		}

		translationFileName = fmt.Sprintf(
			"%s - %s %s by %s",
			episodeEntity.EpisodeLabel,
			display.English.Languages().Name(translationEntity.Variant.Language),
			cases.Title(language.English).String(translationEntity.Variant.Kind.Label()),
			formatAuthorsList(translationEntity.Authors),
		)
	} else {
		translationFileName = fmt.Sprintf(
			"E%05d - %s %s by %s",
			episodeEntity.EpisodeNumber,
			display.English.Languages().Name(translationEntity.Variant.Language),
			cases.Title(language.English).String(translationEntity.Variant.Kind.Label()),
			formatAuthorsList(translationEntity.Authors),
		)

		translationDirectoryRelativePath = showDirectoryName
		translationDirectoryAbsolutePath = filepath.Join(s.downloadsDirectory, translationDirectoryRelativePath)
	}

	translationFileName = filename.Clean(translationFileName)
	translationFileName = strings.TrimSpace(translationFileName)
	translationFileName = strings.Join(strings.Fields(translationFileName), " ")

	fileExists, err := s.fileExists(filepath.Join(translationDirectoryAbsolutePath, translationFileName+".mp4"))
	if err != nil {
		return "", "", fmt.Errorf("failed to check if video file exists: %w", err)
	}

	if fileExists {
		translationFileName = fmt.Sprintf("%s %d", translationFileName, translationEntity.Anime365ID)
	}

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

func (s *Service) UpdateShowMetadata(
	ctx context.Context,
	showFromAnime365 show.Show,
	showFromShikimori show.ShowFromShikimori,
) error {
	embyShowPath, err := s.getEmbyShowPath(showFromAnime365.Anime365ID)
	if err != nil {
		return fmt.Errorf("failed to get show path for %d: %w", showFromAnime365.Anime365ID, err)
	}

	// At this moment Item in Emby may not have any metadata, so we are fetching an item by its path on disk
	itemsResponse, err := s.embyClient.GetItems(ctx, false, []string{"IsFolder"}, s.embyLibraryItemID, embyShowPath, 1)
	if err != nil {
		return ErrEmbyItemNotFound
	}

	if len(itemsResponse.Items) == 0 {
		return fmt.Errorf("no items found for show %d", showFromAnime365.Anime365ID)
	}

	showItem := itemsResponse.Items[0]

	if showFromAnime365.TitleRussian != "" {
		showItem.Name = showFromAnime365.TitleRussian
		showItem.SortName = showFromAnime365.TitleRussian
	} else {
		showItem.Name = showFromAnime365.TitleRomaji
		showItem.SortName = showFromAnime365.TitleRomaji
	}

	showItem.OriginalTitle = showFromAnime365.TitleRomaji

	if showFromAnime365.MyAnimeListScore > 0 {
		showItem.CommunityRating = float32(showFromAnime365.MyAnimeListScore)
	}

	if showFromAnime365.Year > 0 {
		showItem.ProductionYear = int32(showFromAnime365.Year)
	}

	if showFromAnime365.Description != "" {
		showItem.Overview = showFromAnime365.Description
	}

	if len(showFromAnime365.Genres) > 0 {
		showItem.Genres = showFromAnime365.Genres
	}

	if len(showFromAnime365.EpisodePreviews) > 0 {
		if showFromAnime365.IsOngoing {
			showItem.Status = "Continuing"
		} else {
			showItem.Status = "Ended"
		}
	}

	showItem.DisplayOrder = "absolute"

	showItem.ProviderIds = &map[string]string{
		"anime365seriesid": strconv.FormatInt(int64(showFromAnime365.Anime365ID), 10),
	}

	showItem.People = make([]embyclient.BaseItemPerson, 0, len(showFromShikimori.StaffMembers))
	for _, staffMember := range showFromShikimori.StaffMembers {
		switch staffMember.Role {
		case show.StaffRoleDirector:
			showItem.People = append(showItem.People, embyclient.BaseItemPerson{
				Name:  staffMember.Name,
				Type_: new(embyclient.DIRECTOR_PersonType),
			})

		case show.StaffRoleProducer:
			showItem.People = append(showItem.People, embyclient.BaseItemPerson{
				Name:  staffMember.Name,
				Type_: new(embyclient.PRODUCER_PersonType),
			})

		case show.StaffRoleScript:
			showItem.People = append(showItem.People, embyclient.BaseItemPerson{
				Name:  staffMember.Name,
				Type_: new(embyclient.WRITER_PersonType),
			})

		case show.StaffRoleMusic:
			showItem.People = append(showItem.People, embyclient.BaseItemPerson{
				Name:  staffMember.Name,
				Type_: new(embyclient.COMPOSER_PersonType),
			})
		}
	}

	if len(showFromShikimori.Studios) > 0 {
		showItem.Studios = make([]embyclient.NameLongIdPair, 0, len(showFromShikimori.Studios))

		for _, studio := range showFromShikimori.Studios {
			showItem.Studios = append(showItem.Studios, embyclient.NameLongIdPair{
				Name: studio,
			})
		}
	}

	if showFromShikimori.AverageEpisodeDuration != 0 {
		showItem.RunTimeTicks = showFromShikimori.AverageEpisodeDuration.Nanoseconds() / 100
	}

	if !showFromShikimori.PremiereDate.IsZero() {
		showItem.PremiereDate = showFromShikimori.PremiereDate
	}

	if !showFromShikimori.StoppedAiringAt.IsZero() {
		showItem.EndDate = showFromShikimori.StoppedAiringAt
	}

	switch showFromShikimori.AgeRating {
	case show.AgeRatingEnumG:
		showItem.OfficialRating = "G"
	case show.AgeRatingEnumPG:
		showItem.OfficialRating = "PG"
	case show.AgeRatingEnumPG13:
		showItem.OfficialRating = "PG-13"
	case show.AgeRatingEnumR:
		showItem.OfficialRating = "R"
	case show.AgeRatingEnumRPlus:
		showItem.OfficialRating = "RP"
	case show.AgeRatingEnumRx:
		showItem.OfficialRating = "X"
	}

	showItem.LockedFields = []embyclient.MetadataFields{
		embyclient.NAME_MetadataFields,
		embyclient.ORIGINAL_TITLE_MetadataFields,
		embyclient.COMMUNITY_RATING_MetadataFields,
		embyclient.OVERVIEW_MetadataFields,
		embyclient.GENRES_MetadataFields,
		embyclient.SORT_NAME_MetadataFields,
		embyclient.STUDIOS_MetadataFields,
		embyclient.OFFICIAL_RATING_MetadataFields,
		embyclient.CAST_MetadataFields,
	}

	err = s.embyClient.UpdateItem(ctx, showItem.Id, showItem)
	if err != nil {
		return fmt.Errorf("failed to update show item %s: %w", showItem.Id, err)
	}

	return nil
}

func (s *Service) UpdateTranslationMetadata(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeEntity episode.Episode,
	translationEntity episode.Translation,
) error {
	translationPath, err := s.getEmbyTranslationPath(showID, episodeEntity.Anime365ID, translationEntity.Anime365ID)
	if err != nil {
		return fmt.Errorf("failed to get translation path for %d: %w", showID, err)
	}

	itemsResponse, err := s.embyClient.GetItems(
		ctx,
		true,
		[]string{"IsNotFolder"},
		s.embyLibraryItemID,
		translationPath,
		1,
	)
	if err != nil {
		return fmt.Errorf("failed to get items from emby for translation %d: %w", translationEntity.Anime365ID, err)
	}

	if len(itemsResponse.Items) == 0 {
		return ErrEmbyItemNotFound
	}

	translationItem := itemsResponse.Items[0]

	translationItem.Name = episodeEntity.EpisodeLabel

	translationItem.SortName = fmt.Sprintf(
		"Episode %05d, priority %010d",
		episodeEntity.EpisodeNumber,
		translationEntity.Anime365Priority,
	)

	translationItem.ForcedSortName = translationItem.SortName

	if !translationEntity.MarkedAsActiveAt.IsZero() {
		translationItem.PremiereDate = translationEntity.MarkedAsActiveAt
	}

	if episodeEntity.EpisodeNumber < math.MinInt32 || episodeEntity.EpisodeNumber > math.MaxInt32 {
		return errors.New("episode number is out of range int32")
	}

	translationItem.IndexNumber = int32(episodeEntity.EpisodeNumber)

	translationItem.LockedFields = []embyclient.MetadataFields{
		embyclient.NAME_MetadataFields,
		embyclient.SORT_NAME_MetadataFields,
	}

	translationItem.PremiereDate = episodeEntity.FirstUploadedAt

	translationItem.ProviderIds = &map[string]string{
		"anime365seriesid":      strconv.FormatInt(int64(showID), 10),
		"anime365episodeid":     strconv.FormatInt(int64(episodeEntity.Anime365ID), 10),
		"anime365translationid": strconv.FormatInt(int64(translationEntity.Anime365ID), 10),
	}

	err = s.embyClient.UpdateItem(ctx, translationItem.Id, translationItem)
	if err != nil {
		return fmt.Errorf("failed to update translation item %s: %w", translationItem.Id, err)
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
	itemsResponse, err := s.embyClient.GetUserItems(ctx, s.embyUserID, &embyclient.GetUserItemsOptionalParams{
		ParentID: s.embyLibraryItemID,
		Limit:    1,
		AnyProviderIdEquals: map[string]string{
			"anime365seriesid": strconv.FormatInt(int64(showID), 10),
		},
		IsPlayed:  new(true),
		Recursive: new(true),
		SortBy:    []string{"SortName"},
		SortOrder: "Descending",
	})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get emby user items: %w", err)
	}

	if len(itemsResponse.Items) == 0 {
		return 0, 0, ErrEmbyItemNotFound
	}

	item := itemsResponse.Items[0]

	if item.ProviderIds == nil {
		return 0, 0, errors.New("emby item does not have ProviderIds field")
	}

	translationIDStr, ok := (*item.ProviderIds)["anime365translationid"]
	if !ok {
		return 0, 0, errors.New("emby item does not have ProviderIds.anime365translationid field")
	}

	translationID, err := strconv.ParseInt(translationIDStr, 10, 64)
	if err != nil {
		return 0, 0, errors.New("ProviderIds.anime365translationid field is not a number")
	}

	return int64(item.IndexNumber), episode.Anime365TranslationID(translationID), nil
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

func (s *Service) getEmbyShowPath(showID show.Anime365SeriesID) (string, error) {
	showDirectoryName, exists := s.manifestService.GetShowDirectoryName(showID)
	if !exists {
		return "", fmt.Errorf("could not find show %d in manifest", showID)
	}

	return filepath.Join(s.embyLibraryRootDirectory, showDirectoryName), nil
}

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
		return "", fmt.Errorf("could not find translation %d in manifest", translationID)
	}

	return filepath.Join(s.embyLibraryRootDirectory, translationRelativeFilePath), nil
}

func formatAuthorsList(authorsList []string) string {
	if len(authorsList) == 0 {
		return "Unknown"
	}

	if len(authorsList) == 1 {
		return authorsList[0]
	}

	team := authorsList[0]

	replacer := strings.NewReplacer("(", " ", ")", " ", " - ", " ")

	cleanedAuthorsList := make([]string, 0, len(authorsList)-1)
	for _, author := range authorsList[1:] {
		author = replacer.Replace(author)
		author = strings.TrimSpace(author)
		author = strings.Join(strings.Fields(author), " ")

		cleanedAuthorsList = append(cleanedAuthorsList, author)
	}

	cleanedAuthorsListLen := len(cleanedAuthorsList)

	var featuringStr string

	if cleanedAuthorsListLen <= 2 {
		featuringStr = strings.Join(cleanedAuthorsList, " and ")
	} else {
		featuringStr = strings.Join(
			cleanedAuthorsList[:cleanedAuthorsListLen-1],
			", ",
		) + " and " + cleanedAuthorsList[cleanedAuthorsListLen-1]
	}

	return fmt.Sprintf("%s ft. %s", team, featuringStr)
}
