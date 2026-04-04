package show

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

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

func (s *Service) GetSomeShowsFromShikimori(
	ctx context.Context,
	idsMap map[MyAnimeListID]Anime365SeriesID,
) (map[Anime365SeriesID]ShowFromShikimori, error) {
	return s.GetShowsFromShikimori(ctx, randomShowsSubset(
		idsMap,
		shikimoriclient.QueryAnimesMaxLimit,
	))
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

func randomShowsSubset(
	input map[MyAnimeListID]Anime365SeriesID,
	count int,
) map[MyAnimeListID]Anime365SeriesID {
	if count >= len(input) {
		return input
	}

	// collect keys
	keys := make([]MyAnimeListID, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(keys), func(i, j int) {
		keys[i], keys[j] = keys[j], keys[i]
	})

	// build result map
	result := make(map[MyAnimeListID]Anime365SeriesID, count)
	for i := range count {
		k := keys[i]
		result[k] = input[k]
	}

	return result
}
