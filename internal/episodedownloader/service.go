package episodedownloader

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/flaksp/anime365-sidecar/internal/emby"
	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/mylist"
	"github.com/flaksp/anime365-sidecar/internal/scansource"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/anime365client"
	"github.com/flaksp/anime365-sidecar/pkg/downloader"
	"github.com/flaksp/anime365-sidecar/pkg/filesystemutils"
	"golang.org/x/text/language"
)

func NewService(
	myListService *mylist.Service,
	scanSource *scansource.Service,
	episodeService *episode.Service,
	logger *slog.Logger,
	embyService *emby.Service,
	smartDownloader *downloader.SmartDownloader,
	anime365Client *anime365client.Client,
	translations []string,
	downloadVideoTimeout time.Duration,
	temporaryDirectory string,
	preferredTranslationAuthors []string,
) *Service {
	preferredTranslationAuthorsMap := make(map[string]struct{}, len(preferredTranslationAuthors))
	for _, preferredTranslationAuthor := range preferredTranslationAuthors {
		preferredTranslationAuthorsMap[strings.ToLower(preferredTranslationAuthor)] = struct{}{}
	}

	return &Service{
		myListService:                 myListService,
		scanSource:                    scanSource,
		episodeService:                episodeService,
		logger:                        logger,
		embyService:                   embyService,
		downloader:                    smartDownloader,
		anime365Client:                anime365Client,
		translationVariantsToDownload: parseTranslationVariants(translations, logger),
		downloadVideoTimeout:          downloadVideoTimeout,
		temporaryDirectory:            temporaryDirectory,
		preferredTranslationAuthors:   preferredTranslationAuthorsMap,
	}
}

type Service struct {
	myListService                 *mylist.Service
	scanSource                    *scansource.Service
	episodeService                *episode.Service
	logger                        *slog.Logger
	embyService                   *emby.Service
	downloader                    *downloader.SmartDownloader
	anime365Client                *anime365client.Client
	translationVariantsToDownload map[episode.TranslationVariant]struct{}
	preferredTranslationAuthors   map[string]struct{}
	temporaryDirectory            string
	downloadVideoTimeout          time.Duration
}

func (s *Service) ShouldEpisodeBeOnDisk(showID show.Anime365SeriesID, episodeNumber int64) bool {
	if s.scanSource.HasShow(showID) {
		return true
	}

	if s.scanSource.HasList(scansource.SourceListWatching) {
		list := s.myListService.GetWatchingList()

		lastWatchedEpisodeNumber, exists := list[showID]
		if exists && episodeNumber > lastWatchedEpisodeNumber {
			return true
		}
	}

	if s.scanSource.HasList(scansource.SourceListPlanned) {
		list := s.myListService.GetPlannedList()

		lastWatchedEpisodeNumber, exists := list[showID]
		if exists && episodeNumber > lastWatchedEpisodeNumber {
			return true
		}
	}

	return false
}

func (s *Service) DownloadEpisode(
	ctx context.Context,
	showEntity show.Show,
	episodeID episode.Anime365EpisodeID,
) error {
	episodeEntity, err := s.episodeService.GetEpisode(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("could not get episode entity: %w", err)
	}

	for _, translationEntity := range episodeEntity.Translations {
		if !s.shouldDownloadTranslation(translationEntity, episodeEntity.Translations) {
			continue
		}

		if err := s.downloadTranslation(ctx, showEntity, episodeEntity, translationEntity); err != nil {
			s.logger.ErrorContext(
				ctx,
				"Failed to download translation, skipping it",
				slog.Int64("show_id", int64(showEntity.Anime365ID)),
				slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
				slog.Int64("translation_id", int64(translationEntity.Anime365ID)),
				slog.String("error", err.Error()),
			)

			continue
		}
	}

	return nil
}

func (s *Service) downloadTranslation(
	ctx context.Context,
	showEntity show.Show,
	episodeEntity episode.Episode,
	translationEntity episode.Translation,
) error {
	translationMedia, err := s.episodeService.GetTranslationMedia(ctx, translationEntity.Anime365ID)
	if err != nil {
		return fmt.Errorf("could not get translation media: %w", err)
	}

	height, exists := s.embyService.GetTranslationQuality(
		showEntity.Anime365ID,
		episodeEntity.Anime365ID,
		translationEntity.Anime365ID,
	)

	if exists {
		// No need to download media with same or worse quality as already downloaded
		if height >= translationMedia.Height {
			return nil
		}

		s.logger.InfoContext(ctx,
			"Deleting translation because of same translation with better quality found",
			slog.Int64("show_id", int64(showEntity.Anime365ID)),
			slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
			slog.Int64("translation_id", int64(translationEntity.Anime365ID)),
		)

		if err := s.embyService.DeleteTranslation(
			showEntity.Anime365ID,
			episodeEntity.Anime365ID,
			translationEntity.Anime365ID,
		); err != nil {
			return fmt.Errorf("failed to delete translation: %w", err)
		}
	}

	videoTmpFile, err := os.CreateTemp(
		s.temporaryDirectory,
		fmt.Sprintf("anime365-sidecar-translation-%d-*.mp4", translationEntity.Anime365ID),
	)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	videoTmpFilePath := videoTmpFile.Name()
	if err := videoTmpFile.Close(); err != nil {
		s.logger.WarnContext(
			ctx,
			"Closing video tmp file error",
			slog.String("error", err.Error()),
			slog.String("file_path", videoTmpFilePath),
		)
	}

	defer func() {
		if err := filesystemutils.DeleteFileIfExists(videoTmpFilePath); err != nil {
			s.logger.WarnContext(
				ctx,
				"Failed to remove video temp file",
				slog.String("error", err.Error()),
				slog.String("file_path", videoTmpFilePath),
			)
		}
	}()

	videoDownloadCtxWithTimeout, videoDownloadCtxCancel := context.WithTimeout(ctx, s.downloadVideoTimeout)
	defer videoDownloadCtxCancel()

	if err := s.downloader.Download(
		videoDownloadCtxWithTimeout,
		translationMedia.VideoURL,
		videoTmpFilePath,
	); err != nil {
		return fmt.Errorf("failed to download translation media entity video: %w", err)
	}

	var subtitlesBytes []byte

	if translationMedia.SubtitlesURL != nil {
		subtitlesBytes, err = s.anime365Client.GetSubtitles(ctx, translationMedia.SubtitlesURL.Path)
		if err != nil {
			return fmt.Errorf("failed to download translation media entity subtitles: %w", err)
		}
	}

	videoFileAbsolutePath, subtitlesFileAbsoultePath, err := s.embyService.ComputeTranslationFileAbsolutePathsForDownloads(
		showEntity,
		episodeEntity,
		translationEntity,
		translationMedia,
	)
	if err != nil {
		return fmt.Errorf("failed to get compute translation file paths for downloads: %w", err)
	}

	if err := filesystemutils.CopyThenDelete(videoTmpFile.Name(), videoFileAbsolutePath); err != nil {
		return fmt.Errorf("failed to move video file: %w", err)
	}

	if subtitlesBytes != nil && subtitlesFileAbsoultePath != "" {
		subtitlesFile, err := os.Create(subtitlesFileAbsoultePath)
		if err != nil {
			return fmt.Errorf("failed to create subtitles file: %w", err)
		}

		if _, err = subtitlesFile.Write(subtitlesBytes); err != nil {
			return fmt.Errorf("failed to write to subtitles file: %w", err)
		}

		if err := subtitlesFile.Close(); err != nil {
			s.logger.WarnContext(
				ctx,
				"Closing subtitles file error",
				slog.String("error", err.Error()),
				slog.String("file_path", subtitlesFileAbsoultePath),
			)
		}
	}

	if err := s.embyService.SaveTranslationPaths(
		ctx,
		showEntity.Anime365ID,
		episodeEntity.Anime365ID,
		translationEntity.Anime365ID,
		videoFileAbsolutePath,
		subtitlesFileAbsoultePath,
		translationMedia.Height,
	); err != nil {
		return fmt.Errorf("failed to save translation: %w", err)
	}

	return nil
}

func parseTranslationVariants(translations []string, logger *slog.Logger) map[episode.TranslationVariant]struct{} {
	translationVariants := make(map[episode.TranslationVariant]struct{}, len(translations))

	for _, translation := range translations {
		translation = strings.TrimSpace(translation)
		languageAndKind := strings.SplitN(translation, "_", 2)

		if len(languageAndKind) != 2 {
			logger.Warn( // nolint:sloglint
				"Invalid translation variant",
				slog.String("error", "incorrectly formatted"),
				slog.String("translation", translation),
			)

			continue
		}

		languageTag, err := language.Parse(languageAndKind[0])
		if err != nil {
			logger.Warn( // nolint:sloglint
				"Invalid translation variant",
				slog.String("error", err.Error()),
				slog.String("translation", translation),
			)

			continue
		}

		translationVariants[episode.TranslationVariant{
			Kind:     episode.TranslationKind(languageAndKind[1]),
			Language: languageTag,
		}] = struct{}{}
	}

	if len(translationVariants) == 0 {
		logger.Error("No valid translation variants specified") // nolint:sloglint

		return nil
	}

	return translationVariants
}

func (s *Service) shouldDownloadTranslation(
	translationEntity episode.Translation,
	otherTranslationEntities []episode.Translation,
) bool {
	if _, ok := s.translationVariantsToDownload[translationEntity.Variant]; !ok {
		return false
	}

	// download everything if no preferences specified
	if len(s.preferredTranslationAuthors) == 0 {
		return true
	}

	// download if this is preferred translation
	if s.translationHasPreferredAuthor(translationEntity) {
		return true
	}

	foundPreferredInOtherTranslations := false

	for _, otherTranslationEntity := range otherTranslationEntities {
		if otherTranslationEntity.Anime365ID == translationEntity.Anime365ID {
			continue
		}

		if _, ok := s.translationVariantsToDownload[otherTranslationEntity.Variant]; !ok {
			continue
		}

		if s.translationHasPreferredAuthor(otherTranslationEntity) {
			foundPreferredInOtherTranslations = true

			break
		}
	}

	// download everything if all available translations are not preferred
	return !foundPreferredInOtherTranslations
}

func (s *Service) translationHasPreferredAuthor(
	translationEntity episode.Translation,
) bool {
	translationAuthorsMap := make(map[string]struct{}, len(translationEntity.Authors))
	for _, author := range translationEntity.Authors {
		translationAuthorsMap[strings.ToLower(author)] = struct{}{}
	}

	for preferredTranslationAuthor := range s.preferredTranslationAuthors {
		if _, ok := translationAuthorsMap[preferredTranslationAuthor]; ok {
			return true
		}
	}

	return false
}
