package watchednotifier

import (
	"context"
	"errors"
	"log/slog"
	"maps"

	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/mediaserver"
	"github.com/flaksp/anime365-sidecar/internal/mylist"
	"github.com/flaksp/anime365-sidecar/internal/scansource"
	"github.com/flaksp/anime365-sidecar/internal/show"
)

func NewService(
	logger *slog.Logger,
	myListService *mylist.Service,
	mediaServer mediaserver.Server,
	scanSource *scansource.Service,
	episodeService *episode.Service,
) *Service {
	return &Service{
		logger:         logger,
		myListService:  myListService,
		mediaServer:    mediaServer,
		scanSource:     scanSource,
		episodeService: episodeService,
	}
}

type Service struct {
	logger         *slog.Logger
	myListService  *mylist.Service
	mediaServer    mediaserver.Server
	scanSource     *scansource.Service
	episodeService *episode.Service
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
		lastWatchedEpisodeNumberInLibrary, translationID, err := s.mediaServer.GetLastWatchedEpisodeNumber(ctx, showID)
		if err != nil {
			if errors.Is(err, mediaserver.ErrItemNotFound) {
				s.logger.DebugContext(
					ctx,
					"Translation not found in media server items, probably will be indexed later",
					slog.Int64("show_id", int64(showID)),
				)

				continue
			}

			s.logger.ErrorContext(
				ctx,
				"Failed to get last watched episode number in media server library",
				slog.Int64("show_id", int64(showID)),
				slog.String("error", err.Error()),
			)

			continue
		}

		lastWatchedEpisodeNumberOnAnime365 := s.myListService.GetLastWatchedEpisodeNumber(showID)

		if lastWatchedEpisodeNumberOnAnime365 >= lastWatchedEpisodeNumberInLibrary {
			s.logger.DebugContext(
				ctx,
				"Skipping show because list in Anime 365 has larger episode number than last watched in media server",
				slog.Int64("show_id", int64(showID)),
				slog.Int64("last_watched_episode_number_on_anime365", lastWatchedEpisodeNumberOnAnime365),
				slog.Int64("last_watched_episode_number_in_library", lastWatchedEpisodeNumberInLibrary),
			)

			continue
		}

		if err := s.episodeService.MarkTranslationAsWatched(ctx, translationID); err != nil {
			s.logger.ErrorContext(
				ctx,
				"Failed to notify Anime 365 about watched episode translation",
				slog.Int64("show_id", int64(showID)),
				slog.Int64("translation_id", int64(translationID)),
				slog.String("error", err.Error()),
			)

			continue
		}
	}

	return nil
}
