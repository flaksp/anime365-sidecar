package librarycleanup

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/flaksp/anime365-sidecar/internal/emby"
	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/notificationsender"
	"github.com/flaksp/anime365-sidecar/internal/show"
)

func NewService(
	episodeService *episode.Service,
	showService *show.Service,
	embyService *emby.Service,
	logger *slog.Logger,
	notificationSender *notificationsender.Service,
) *Service {
	return &Service{
		episodeService:     episodeService,
		showService:        showService,
		embyService:        embyService,
		logger:             logger,
		notificationSender: notificationSender,
	}
}

type Service struct {
	episodeService     *episode.Service
	showService        *show.Service
	embyService        *emby.Service
	logger             *slog.Logger
	notificationSender *notificationsender.Service
}

func (s *Service) RunOnce(ctx context.Context) error {
	for showID, episodes := range s.embyService.GetIDs() {
		var showEntity show.Show

		for episodeID, translations := range episodes {
			if len(translations) == 0 {
				continue
			}

			if showEntity.Anime365ID == 0 {
				var err error

				showEntity, err = s.showService.GetShow(ctx, showID)
				if err != nil {
					s.logger.ErrorContext(ctx, "Failed to get show for cleanup, skipping it",
						slog.Int64("show_id", int64(showID)),
						slog.String("error", err.Error()),
					)

					break
				}
			}

			episodeEntity, err := s.episodeService.GetEpisode(ctx, episodeID)
			if err == nil {
				err = s.deleteTranslationsRemovedFromAnime365(ctx, showEntity, episodeEntity, translations)
			}

			if err != nil {
				s.logger.ErrorContext(ctx, "Failed to clean up downloaded episode, skipping it",
					slog.Int64("show_id", int64(showID)),
					slog.Int64("episode_id", int64(episodeID)),
					slog.String("error", err.Error()),
				)
			}
		}
	}

	return nil
}

func (s *Service) deleteTranslationsRemovedFromAnime365(
	ctx context.Context,
	showEntity show.Show,
	episodeEntity episode.Episode,
	downloadedTranslationIDs map[episode.Anime365TranslationID]struct{},
) error {
	availableIDs := availableTranslationIDs(episodeEntity.Translations)
	if episodeEntity.IsUnavailable {
		// An explicitly inactive episode makes all translations unavailable,
		// even when the API omits the translations collection.
		availableIDs = make(map[episode.Anime365TranslationID]struct{})
	}

	removedTranslationIDs := findRemovedTranslationIDs(
		downloadedTranslationIDs,
		availableIDs,
	)
	if len(removedTranslationIDs) == 0 {
		return nil
	}

	deletedAnyTranslation := false

	for _, translationID := range removedTranslationIDs {
		deleted, err := s.embyService.DeleteTranslationIfNotPlaying(
			ctx,
			showEntity.Anime365ID,
			episodeEntity.Anime365ID,
			translationID,
		)
		if err != nil {
			return fmt.Errorf("failed to delete translation or episode removed or hidden on anime 365: %w", err)
		}

		if !deleted {
			s.logger.InfoContext(
				ctx,
				"Deferring deletion because translation is being watched in Emby",
				slog.Int64("show_id", int64(showEntity.Anime365ID)),
				slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
				slog.Int64("translation_id", int64(translationID)),
			)

			continue
		}

		s.logger.InfoContext(
			ctx,
			"Deleted translation because it or its episode was removed or hidden on Anime 365",
			slog.Int64("show_id", int64(showEntity.Anime365ID)),
			slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
			slog.Int64("translation_id", int64(translationID)),
		)

		if s.notificationSender != nil {
			showName := showEntity.TitleRussian
			if showName == "" {
				showName = showEntity.TitleRomaji
			}

			if err := s.notificationSender.TranslationDeleted(
				ctx,
				showName,
				episodeEntity.EpisodeLabel,
				translationID,
				episodeEntity.Translations[translationID],
			); err != nil {
				s.logger.WarnContext(
					ctx,
					"Error sending translation deleted notification to user",
					slog.Int64("show_id", int64(showEntity.Anime365ID)),
					slog.Int64("episode_id", int64(episodeEntity.Anime365ID)),
					slog.Int64("translation_id", int64(translationID)),
					slog.String("error", err.Error()),
				)
			}
		}

		deletedAnyTranslation = true
	}

	if deletedAnyTranslation {
		if err := s.embyService.RefreshLibrary(ctx); err != nil {
			s.logger.WarnContext(
				ctx,
				"Failed to refresh Emby library after deleting removed translations",
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

func availableTranslationIDs(
	translations map[episode.Anime365TranslationID]episode.Translation,
) map[episode.Anime365TranslationID]struct{} {
	if translations == nil {
		return nil
	}

	translationIDs := make(map[episode.Anime365TranslationID]struct{}, len(translations))
	for translationID, translationEntity := range translations {
		if translationEntity.IsUnavailable {
			continue
		}

		translationIDs[translationID] = struct{}{}
	}

	return translationIDs
}

func findRemovedTranslationIDs(
	downloadedTranslationIDs map[episode.Anime365TranslationID]struct{},
	availableTranslationIDs map[episode.Anime365TranslationID]struct{},
) []episode.Anime365TranslationID {
	// A nil collection means Anime 365 did not provide translation availability,
	// so it is not safe to infer that every downloaded translation was removed.
	if availableTranslationIDs == nil {
		return nil
	}

	removedTranslationIDs := make([]episode.Anime365TranslationID, 0)

	for translationID := range downloadedTranslationIDs {
		if _, exists := availableTranslationIDs[translationID]; !exists {
			removedTranslationIDs = append(removedTranslationIDs, translationID)
		}
	}

	slices.Sort(removedTranslationIDs)

	return removedTranslationIDs
}
