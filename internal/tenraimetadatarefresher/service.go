package tenraimetadatarefresher

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/mediaserver"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/tenraiclient"
)

type Service struct {
	showService  *show.Service
	mediaServer  mediaserver.Server
	tenraiClient *tenraiclient.ClientWithResponses
	logger       *slog.Logger
}

func NewService(
	showService *show.Service,
	mediaServer mediaserver.Server,
	tenraiClient *tenraiclient.ClientWithResponses,
	logger *slog.Logger,
) *Service {
	return &Service{
		showService:  showService,
		mediaServer:  mediaServer,
		tenraiClient: tenraiClient,
		logger:       logger,
	}
}

func (s *Service) RunOnce(ctx context.Context) error {
	ids, err := s.mediaServer.GetIDsWithMetadata(ctx)
	if err != nil {
		return fmt.Errorf("getting initialized episodes for Tenrai metadata: %w", err)
	}

	for showID, episodes := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}

		s.refreshShow(ctx, showID, episodes)
	}

	return nil
}

func (s *Service) refreshShow(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodes map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{},
) {
	showEntity, err := s.showService.GetShow(ctx, showID)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to get Anime365 episode numbers for Tenrai metadata",
			slog.Int64("show_id", int64(showID)),
			slog.String("error", err.Error()),
		)

		return
	}

	if showEntity.MyAnimeListID <= 0 {
		return
	}

	regularEpisodes := make(map[episode.Anime365EpisodeID]int64, len(episodes))
	for _, preview := range showEntity.EpisodePreviews {
		if _, exists := episodes[preview.Anime365ID]; exists && preview.EpisodeNumber > 0 && !preview.IsTrailer {
			regularEpisodes[preview.Anime365ID] = preview.EpisodeNumber
		}
	}

	if len(regularEpisodes) == 0 {
		return
	}

	catalog, err := s.fetchTenraiEpisodes(ctx, showEntity.MyAnimeListID)
	if err != nil && ctx.Err() == nil {
		s.logger.WarnContext(ctx, "Failed to get episode metadata from Tenrai",
			slog.Int64("show_id", int64(showID)),
			slog.Int64("my_anime_list_id", int64(showEntity.MyAnimeListID)),
			slog.String("error", err.Error()),
		)
	}

	for episodeID, episodeNumber := range regularEpisodes {
		metadata, exists := catalog[episodeNumber]
		if !exists || metadata == (episode.CatalogMetadata{}) {
			continue
		}

		for translationID := range episodes[episodeID] {
			if err := ctx.Err(); err != nil {
				return
			}

			err := s.mediaServer.UpdateTranslationCatalogMetadata(ctx, showID, episodeID, translationID, metadata)
			if err != nil {
				s.logger.WarnContext(ctx, "Failed to update episode metadata from Tenrai",
					slog.Int64("show_id", int64(showID)),
					slog.Int64("episode_id", int64(episodeID)),
					slog.Int64("translation_id", int64(translationID)),
					slog.String("error", err.Error()),
				)
			}
		}
	}
}

func (s *Service) fetchTenraiEpisodes(
	ctx context.Context,
	malID show.MyAnimeListID,
) (map[int64]episode.CatalogMetadata, error) {
	metadata := make(map[int64]episode.CatalogMetadata)

	for page := 1; ; page++ {
		response, err := s.tenraiClient.GetAnimeByIDEpisodesWithResponse(
			ctx, int(malID), &tenraiclient.GetAnimeByIDEpisodesParams{Page: &page},
		)
		if err != nil {
			return metadata, fmt.Errorf("requesting episode list page %d: %w", page, err)
		}

		if page == 1 && response.StatusCode() == http.StatusNotFound {
			return metadata, nil
		}

		if response.JSON200 == nil || response.JSON200.Data == nil {
			return metadata, fmt.Errorf("unexpected Tenrai response on page %d: %s", page, response.Status())
		}

		for _, row := range response.JSON200.Data {
			if row.MalID == nil || *row.MalID <= 0 {
				continue
			}

			entry := episode.CatalogMetadata{Filler: row.Filler, Recap: row.Recap}
			// Tenrai reports a five-star average. Media servers use a ten-point rating.
			if score := row.Score; score != nil && *score >= 1 && *score <= 5 {
				entry.CommunityRating = new(*score * 2)
			}

			if row.Title != nil {
				entry.Title = strings.TrimSpace(*row.Title)
			}

			if row.Synopsis != nil {
				entry.Synopsis = strings.TrimSpace(*row.Synopsis)
			}

			if row.Aired != nil && *row.Aired != "" {
				airedAt, err := time.Parse(time.RFC3339, *row.Aired)
				if err != nil {
					s.logger.WarnContext(ctx, "Invalid Tenrai episode air date",
						slog.Int64("my_anime_list_id", int64(malID)),
						slog.Int("episode_number", *row.MalID),
						slog.String("error", err.Error()),
					)
				}

				entry.AiredAt = airedAt
			}

			metadata[int64(*row.MalID)] = entry
		}

		if !response.JSON200.Pagination.HasNextPage {
			return metadata, nil
		}

		if page >= response.JSON200.Pagination.LastVisiblePage || len(response.JSON200.Data) == 0 {
			return metadata, errors.New("inconsistent Tenrai episode pagination")
		}
	}
}
