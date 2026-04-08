package show

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/flaksp/anime365-sidecar/pkg/anime365client"
	"github.com/flaksp/anime365-sidecar/pkg/embyclient"
	"github.com/flaksp/anime365-sidecar/pkg/shikimoriclient"
)

func NewService(
	anime365Client *anime365client.Client,
	embyClient *embyclient.Client,
	shikimoriClient *shikimoriclient.Client,
	logger *slog.Logger,
) *Service {
	return &Service{
		anime365Client:  anime365Client,
		shikimoriClient: shikimoriClient,
		embyClient:      embyClient,
		logger:          logger,
	}
}

type Service struct {
	anime365Client  *anime365client.Client
	embyClient      *embyclient.Client
	shikimoriClient *shikimoriclient.Client
	logger          *slog.Logger
}

func (s *Service) GetShow(ctx context.Context, showID Anime365SeriesID) (Show, error) {
	seriesDTO, err := s.anime365Client.GetSeries(ctx, int64(showID))
	if err != nil {
		return Show{}, fmt.Errorf("getting series from anime 365 api: %w", err)
	}

	showEntity, err := NewShow(seriesDTO)
	if err != nil {
		return Show{}, fmt.Errorf("creating show entity from anime 365 api dto: %w", err)
	}

	return showEntity, nil
}

func (s *Service) GetShowFromShikimori(
	ctx context.Context,
	id MyAnimeListID,
) (ShowFromShikimori, error) {
	shows, err := s.GetShowsFromShikimori(ctx, map[MyAnimeListID]Anime365SeriesID{id: 0})
	if err != nil {
		return ShowFromShikimori{}, err
	}

	if len(shows) == 0 {
		return ShowFromShikimori{}, errors.New("no shikimori shows found")
	}

	return shows[0], nil
}

func (s *Service) GetShowsFromShikimori(
	ctx context.Context,
	idsMap map[MyAnimeListID]Anime365SeriesID,
) (map[Anime365SeriesID]ShowFromShikimori, error) {
	if len(idsMap) == 0 {
		return make(map[Anime365SeriesID]ShowFromShikimori), nil
	}

	malIDs := make([]int64, 0, len(idsMap))

	for malID := range idsMap {
		malIDs = append(malIDs, int64(malID))
	}

	animeDTOs, err := s.shikimoriClient.GetAnimes(ctx, malIDs)
	if err != nil {
		return nil, fmt.Errorf("getting animes from shikimori api: %w", err)
	}

	res := make(map[Anime365SeriesID]ShowFromShikimori, len(animeDTOs))

	for _, animeDTO := range animeDTOs {
		showFromShikimori, err := NewShowFromShikimoriFromDTO(animeDTO)
		if err != nil {
			s.logger.WarnContext(ctx, "Failed to normalize Shikimori Anime DTO", slog.String("error", err.Error()))

			continue
		}

		anime365ID, ok := idsMap[showFromShikimori.ID]
		if !ok {
			s.logger.WarnContext(ctx, "Shikimori API returned show that wasn't requested")

			continue
		}

		res[anime365ID] = showFromShikimori
	}

	return res, nil
}
