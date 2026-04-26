package librarymetadatarefresher

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/flaksp/anime365-sidecar/internal/emby"
	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/downloader"
	"github.com/flaksp/anime365-sidecar/pkg/filesystemutils"
)

func NewService(
	showService *show.Service,
	episodeService *episode.Service,
	embyService *emby.Service,
	smartDownloader *downloader.SmartDownloader,
	logger *slog.Logger,
	downloadImageTimeout time.Duration,
) *Service {
	return &Service{
		showService:          showService,
		episodeService:       episodeService,
		embyService:          embyService,
		downloader:           smartDownloader,
		logger:               logger,
		downloadImageTimeout: downloadImageTimeout,
	}
}

type Service struct {
	showService          *show.Service
	episodeService       *episode.Service
	embyService          *emby.Service
	downloader           *downloader.SmartDownloader
	logger               *slog.Logger
	downloadImageTimeout time.Duration
}

func (s *Service) RunOnceForItemsWithoutMetadata(ctx context.Context) error {
	ids, err := s.embyService.GetIDsWithoutMetadataFromEmbyLibrary(ctx)
	if err != nil {
		return fmt.Errorf("failed to get item ids without metadata from emby library: %w", err)
	}

	for showID, items := range ids {
		showEntity, err := s.showService.GetShow(ctx, showID)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to get show, it was not updated",
				slog.Int64("show_id", int64(showID)),
				slog.String("error", err.Error()),
			)

			continue
		}

		if err = s.embyService.InitialUpdateShowMetadataWithAnime365Metadata(ctx, showEntity); err != nil {
			s.logger.WarnContext(
				ctx,
				"Failed to perform initial update show in Emby with Anime 365 metadata, this metadata was not updated",
				slog.Int64("show_id", int64(showEntity.Anime365ID)),
				slog.String("error", err.Error()),
			)
		}

		err = s.downloadPosterIfNotExists(ctx, showEntity)
		if err != nil {
			s.logger.WarnContext(ctx, "Failed to download poster",
				slog.Int64("show_id", int64(showEntity.Anime365ID)),
				slog.String("error", err.Error()),
			)
		}

		for episodeID, items := range items {
			episodeEntity, err := s.episodeService.GetEpisode(ctx, episodeID)
			if err != nil {
				s.logger.ErrorContext(ctx, "Failed to get episode, it will not be updated",
					slog.Int64("show_id", int64(showEntity.Anime365ID)),
					slog.Int64("episode_id", int64(episodeID)),
					slog.String("error", err.Error()),
				)

				continue
			}

			for translationID := range items {
				translationEntity, err := s.episodeService.GetTranslation(ctx, translationID)
				if err != nil {
					s.logger.ErrorContext(ctx, "Failed to get translation, it will not be updated",
						slog.Int64("show_id", int64(showEntity.Anime365ID)),
						slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
						slog.Int64("translation_id", int64(translationID)),
						slog.String("error", err.Error()),
					)

					continue
				}

				if err = s.embyService.InitialUpdateTranslationMetadataWithAnime365Metadata(
					ctx,
					showID,
					episodeEntity,
					translationEntity,
				); err != nil {
					s.logger.ErrorContext(
						ctx,
						"Failed to perform initial update translation in Emby with Anime 365 metadata, it was not updated",
						slog.Int64("show_id", int64(showEntity.Anime365ID)),
						slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
						slog.Int64("translation_id", int64(translationEntity.Anime365ID)),
						slog.String("error", err.Error()),
					)

					continue
				}
			}
		}
	}

	return nil
}

func (s *Service) RunOnceForItemsWithMetadata(ctx context.Context) error {
	ids, err := s.embyService.GetIDsWithMetadataFromEmbyLibrary(ctx)
	if err != nil {
		return fmt.Errorf("failed to get item ids with metadata from emby library: %w", err)
	}

	for showID, items := range ids {
		showEntity, err := s.showService.GetShow(ctx, showID)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to get show, it was not updated",
				slog.Int64("show_id", int64(showID)),
				slog.String("error", err.Error()),
			)

			continue
		}

		if err = s.embyService.UpdateShowMetadataWithAnime365Metadata(ctx, showEntity); err != nil {
			s.logger.WarnContext(
				ctx,
				"Failed to update show in Emby with Anime 365 metadata, this metadata was not updated",
				slog.Int64("show_id", int64(showEntity.Anime365ID)),
				slog.String("error", err.Error()),
			)
		}

		showFromShikimori, err := s.showService.GetShowFromShikimori(ctx, showEntity.MyAnimeListID)
		if err != nil {
			s.logger.WarnContext(
				ctx,
				"Failed to get show from Shikimori, show will be updated without this metadata",
				slog.Int64("show_id", int64(showEntity.Anime365ID)),
				slog.Int64("show_my_anime_list_id", int64(showEntity.MyAnimeListID)),
				slog.String("error", err.Error()),
			)
		}

		if err = s.embyService.UpdateShowMetadataWithShikimoriMetadata(ctx, showID, showFromShikimori); err != nil {
			s.logger.WarnContext(
				ctx,
				"Failed to update show in Emby with Shikimori metadata, this metadata was not updated",
				slog.Int64("show_id", int64(showEntity.Anime365ID)),
				slog.String("error", err.Error()),
			)
		}

		err = s.downloadPosterIfNotExists(ctx, showEntity)
		if err != nil {
			s.logger.WarnContext(ctx, "Failed to download poster",
				slog.Int64("show_id", int64(showEntity.Anime365ID)),
				slog.String("error", err.Error()),
			)
		}

		if showFromShikimori.Screenshots != nil {
			for _, screenshot := range showFromShikimori.Screenshots {
				err := s.downloadBackdropIfNotExists(ctx, showID, screenshot.ImageURL, screenshot.ID)
				if err != nil {
					s.logger.WarnContext(
						ctx,
						"Failed to download screenshot as backdrop",
						slog.Int64("show_id", int64(showEntity.Anime365ID)),
						slog.Int64("show_my_anime_list_id", int64(showEntity.MyAnimeListID)),
						slog.String("error", err.Error()),
					)
				}
			}
		}

		for episodeID, items := range items {
			episodeEntity, err := s.episodeService.GetEpisode(ctx, episodeID)
			if err != nil {
				s.logger.ErrorContext(ctx, "Failed to get episode, it will not be updated",
					slog.Int64("show_id", int64(showEntity.Anime365ID)),
					slog.Int64("episode_id", int64(episodeID)),
					slog.String("error", err.Error()),
				)

				continue
			}

			var episodeMetadataFromJikan episode.MetadataFromJikan

			if episodeEntity.EpisodeNumber > 0 {
				episodeMetadataFromJikan, err = s.episodeService.GetEpisodeMetadataFromJikan(
					ctx,
					int64(showEntity.MyAnimeListID),
					episodeEntity.EpisodeNumber,
				)
				if errors.Is(err, episode.ErrJikanEpisodeNotFound) {
					s.logger.DebugContext(
						ctx,
						"Episode not found on Jikan, all translations will be updated without this metadata",
						slog.Int64("show_id", int64(showEntity.Anime365ID)),
						slog.Int64("show_my_anime_list_id", int64(showEntity.MyAnimeListID)),
						slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
						slog.Int64("episode_number", episodeEntity.EpisodeNumber),
					)
				} else if err != nil {
					s.logger.WarnContext(
						ctx,
						"Failed to get episode metadata from Jikan, all translations will be updated without this metadata",
						slog.Int64("show_id", int64(showEntity.Anime365ID)),
						slog.Int64("show_my_anime_list_id", int64(showEntity.MyAnimeListID)),
						slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
						slog.Int64("episode_number", episodeEntity.EpisodeNumber),
						slog.String("error", err.Error()),
					)
				}
			}

			for translationID := range items {
				translationEntity, err := s.episodeService.GetTranslation(ctx, translationID)
				if err != nil {
					s.logger.ErrorContext(ctx, "Failed to get translation, it will not be updated",
						slog.Int64("show_id", int64(showEntity.Anime365ID)),
						slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
						slog.Int64("translation_id", int64(translationID)),
						slog.String("error", err.Error()),
					)

					continue
				}

				if err = s.embyService.UpdateTranslationMetadataWithAnime365Metadata(
					ctx,
					showID,
					episodeEntity,
					translationEntity,
				); err != nil {
					s.logger.ErrorContext(
						ctx,
						"Failed to update translation in Emby with Anime 365 metadata, it was not updated",
						slog.Int64("show_id", int64(showEntity.Anime365ID)),
						slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
						slog.Int64("translation_id", int64(translationEntity.Anime365ID)),
						slog.String("error", err.Error()),
					)

					continue
				}

				if episodeMetadataFromJikan.Title != "" {
					if err = s.embyService.UpdateTranslationMetadataWithJikanMetadata(
						ctx,
						showID,
						episodeID,
						translationEntity.Anime365ID,
						episodeMetadataFromJikan,
					); err != nil {
						s.logger.ErrorContext(
							ctx,
							"Failed to update translation in Emby with Jikan metadata, it was not updated",
							slog.Int64("show_id", int64(showEntity.Anime365ID)),
							slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
							slog.Int64("translation_id", int64(translationEntity.Anime365ID)),
							slog.String("error", err.Error()),
						)

						continue
					}
				}
			}
		}
	}

	return nil
}

func (s *Service) downloadPosterIfNotExists(
	ctx context.Context,
	showEntity show.Show,
) error {
	if showEntity.PosterURL == nil {
		return nil
	}

	exists, err := s.embyService.IsPosterExists(showEntity.Anime365ID, showEntity.PosterURL)
	if err != nil {
		return fmt.Errorf("failed to check if show poster exists: %w", err)
	}

	if exists {
		return nil
	}

	posterTmpFile, err := os.CreateTemp(
		"",
		fmt.Sprintf("anime365-sidecar-poster-%d-*%s", showEntity.Anime365ID, filepath.Ext(showEntity.PosterURL.Path)),
	)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	defer func() {
		if err := posterTmpFile.Close(); err != nil {
			s.logger.WarnContext(ctx, "Closing poster tmp file error", slog.String("error", err.Error()))
		}

		if err := os.Remove(posterTmpFile.Name()); err != nil && !os.IsNotExist(err) {
			s.logger.WarnContext(
				ctx,
				"Failed to remove poster temp file",
				slog.String("error", err.Error()),
				slog.String("file_path", posterTmpFile.Name()),
			)
		}
	}()

	imageDownloadCtxWithTimeout, imageDownloadCtxCancel := context.WithTimeout(ctx, s.downloadImageTimeout)
	defer imageDownloadCtxCancel()

	err = s.downloader.Download(imageDownloadCtxWithTimeout, showEntity.PosterURL, posterTmpFile)
	if err != nil {
		return fmt.Errorf("failed to download poster: %w", err)
	}

	posterFileAbsolutePath, err := s.embyService.ComputePosterFileAbsolutePath(
		showEntity.Anime365ID,
		showEntity.PosterURL,
	)
	if err != nil {
		return fmt.Errorf("failed to compute poster file name: %w", err)
	}

	err = os.Rename(posterTmpFile.Name(), posterFileAbsolutePath)
	if errors.Is(err, syscall.EXDEV) {
		err = filesystemutils.CopyThenDelete(posterTmpFile.Name(), posterFileAbsolutePath)
	}

	if err != nil {
		return fmt.Errorf("failed to move poster file: %w", err)
	}

	return nil
}

func (s *Service) downloadBackdropIfNotExists(
	ctx context.Context,
	showID show.Anime365SeriesID,
	imageURL *url.URL,
	screenshotID string,
) error {
	if s.embyService.IsBackdropExists(showID, screenshotID) {
		return nil
	}

	backdropTmpFile, err := os.CreateTemp(
		"",
		fmt.Sprintf("anime365-sidecar-backdrop-%s-*%s", screenshotID, filepath.Ext(imageURL.Path)),
	)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	defer func() {
		if err := backdropTmpFile.Close(); err != nil {
			s.logger.WarnContext(ctx, "Closing backdrop tmp file error", slog.String("error", err.Error()))
		}

		if err := os.Remove(backdropTmpFile.Name()); err != nil && !os.IsNotExist(err) {
			s.logger.WarnContext(
				ctx,
				"Failed to remove backdrop temp file",
				slog.String("error", err.Error()),
				slog.String("file_path", backdropTmpFile.Name()),
			)
		}
	}()

	imageDownloadCtxWithTimeout, imageDownloadCtxCancel := context.WithTimeout(ctx, s.downloadImageTimeout)
	defer imageDownloadCtxCancel()

	err = s.downloader.Download(imageDownloadCtxWithTimeout, imageURL, backdropTmpFile)
	if err != nil {
		return fmt.Errorf("failed to download backdrop: %w", err)
	}

	backdropFileAbsolutePath, err := s.embyService.ComputeBackdropFileAbsolutePath(showID, imageURL)
	if err != nil {
		return fmt.Errorf("failed to compute backdrop file name: %w", err)
	}

	err = os.Rename(backdropTmpFile.Name(), backdropFileAbsolutePath)
	if errors.Is(err, syscall.EXDEV) {
		err = filesystemutils.CopyThenDelete(backdropTmpFile.Name(), backdropFileAbsolutePath)
	}

	if err != nil {
		return fmt.Errorf("failed to move backdrop file: %w", err)
	}

	err = s.embyService.AddBackdrop(showID, screenshotID)
	if err != nil {
		return fmt.Errorf("failed to add backdrop: %w", err)
	}

	return nil
}
