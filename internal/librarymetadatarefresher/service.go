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

func (s *Service) RunOnce(ctx context.Context) error {
	myAnimeListIDToShowIDMap := s.embyService.GetMyAnimeListIDToShowIDMap()

	showsFromShikimori, err := s.showService.GetSomeShowsFromShikimori(ctx, myAnimeListIDToShowIDMap)
	if err != nil {
		return fmt.Errorf("getting shows from shikimori: %w", err)
	}

	for showID, items := range s.embyService.GetIDs() {
		showEntity, err := s.showService.GetShow(ctx, showID)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to get show", slog.String("error", err.Error()))

			continue
		}

		err = s.downloadPosterIfNotExists(ctx, showEntity)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to download poster", slog.String("error", err.Error()))

			continue
		}

		if showsFromShikimori[showID].Screenshots != nil {
			for _, screenshot := range showsFromShikimori[showID].Screenshots {
				err := s.downloadBackdropIfNotExists(ctx, showID, screenshot.ImageURL, screenshot.ID)
				if err != nil {
					s.logger.ErrorContext(
						ctx,
						"Failed to download screenshot as backdrop",
						slog.String("error", err.Error()),
					)
				}
			}
		}

		err = s.embyService.UpdateShowMetadata(
			ctx,
			showEntity,
			showsFromShikimori[showID],
		)
		if err != nil {
			if errors.Is(err, emby.ErrEmbyItemNotFound) {
				s.logger.DebugContext(
					ctx,
					"Show not found in Emby items, probably will be indexed later",
					slog.Int64("show_id", int64(showEntity.Anime365ID)),
				)

				continue
			}

			s.logger.ErrorContext(ctx, "Failed to update show", slog.String("error", err.Error()))

			continue
		}

		for episodeID, items := range items {
			episodeEntity, err := s.episodeService.GetEpisode(ctx, episodeID)
			if err != nil {
				s.logger.ErrorContext(ctx, "Failed to get episode", slog.String("error", err.Error()))

				continue
			}

			for translationID := range items {
				translationEntity, err := s.episodeService.GetTranslation(ctx, translationID)
				if err != nil {
					s.logger.ErrorContext(ctx, "Failed to get translation", slog.String("error", err.Error()))

					continue
				}

				err = s.embyService.UpdateTranslationMetadata(
					ctx,
					showID,
					episodeEntity,
					translationEntity,
				)
				if err != nil {
					if errors.Is(err, emby.ErrEmbyItemNotFound) {
						s.logger.DebugContext(
							ctx,
							"Translation not found in Emby items, probably will be indexed later",
							slog.Int64("show_id", int64(showEntity.Anime365ID)),
							slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
							slog.Int64("translation_id", int64(translationEntity.Anime365ID)),
						)

						continue
					}

					s.logger.ErrorContext(ctx, "Failed to update translation", slog.String("error", err.Error()))
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
		err := posterTmpFile.Close()
		if err != nil {
			s.logger.WarnContext(ctx, "Closing poster tmp file error", slog.String("error", err.Error()))
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
		err := backdropTmpFile.Close()
		if err != nil {
			s.logger.WarnContext(ctx, "Closing backdrop tmp file error", slog.String("error", err.Error()))
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
