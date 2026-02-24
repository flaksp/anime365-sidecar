package showdownloader

import (
	"context"
	"fmt"
	"log/slog"
	"maps"

	"github.com/flaksp/anime365-emby/internal/emby"
	"github.com/flaksp/anime365-emby/internal/episodedownloader"
	"github.com/flaksp/anime365-emby/internal/mylist"
	"github.com/flaksp/anime365-emby/internal/scansource"
	"github.com/flaksp/anime365-emby/internal/show"
)

func NewService(
	showService *show.Service,
	logger *slog.Logger,
	myListService *mylist.Service,
	scanSource *scansource.Service,
	embyService *emby.Service,
	episodeDownloader *episodedownloader.Service,
) *Service {
	return &Service{
		showService:       showService,
		logger:            logger,
		myListService:     myListService,
		scanSource:        scanSource,
		embyService:       embyService,
		episodeDownloader: episodeDownloader,
	}
}

type Service struct {
	showService       *show.Service
	logger            *slog.Logger
	myListService     *mylist.Service
	scanSource        *scansource.Service
	embyService       *emby.Service
	episodeDownloader *episodedownloader.Service
}

func (s *Service) RunOnce(
	ctx context.Context,
) error {
	showIDs := make(map[show.Anime365SeriesID]struct{})

	if s.scanSource.HasList(scansource.SourceListWatching) {
		list := s.myListService.GetWatchingList()

		for showID := range list {
			showIDs[showID] = struct{}{}
		}
	}

	if s.scanSource.HasList(scansource.SourceListPlanned) {
		list := s.myListService.GetPlannedList()

		for showID := range list {
			showIDs[showID] = struct{}{}
		}
	}

	maps.Copy(showIDs, s.scanSource.GetForcedShowIDs())

	for showID := range showIDs {
		err := s.downloadShow(
			ctx,
			showID,
		)
		if err != nil {
			s.logger.ErrorContext(
				ctx,
				"Failed to download show, skipping it",
				slog.Int64("show_id", int64(showID)),
				slog.String("error", err.Error()),
			)

			continue
		}
	}

	return nil
}

func (s *Service) downloadShow(
	ctx context.Context,
	showID show.Anime365SeriesID,
) error {
	showEntity, err := s.showService.GetShow(ctx, showID)
	if err != nil {
		return fmt.Errorf("failed to get show: %w", err)
	}

	err = s.embyService.CreateShowIfNotExists(showID, showEntity.TitleRomaji, showEntity.MyAnimeListID)
	if err != nil {
		return fmt.Errorf("failed to create show: %w", err)
	}

	for _, episodePreview := range showEntity.EpisodePreviews {
		if !s.episodeDownloader.ShouldEpisodeBeOnDisk(showID, episodePreview.EpisodeNumber) {
			continue
		}

		err = s.episodeDownloader.DownloadEpisode(ctx, showEntity, episodePreview.Anime365ID)
		if err != nil {
			s.logger.ErrorContext(
				ctx,
				"Failed to download episode, skipping it",
				slog.Int64("show_id", int64(showID)),
				slog.Int64("episode_id", int64(episodePreview.Anime365ID)),
				slog.String("error", err.Error()),
			)

			continue
		}
	}

	return nil
}
