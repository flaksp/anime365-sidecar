package episode

import (
	"context"
	"errors"
	"fmt"

	"github.com/flaksp/anime365-sidecar/pkg/anime365client"
	"github.com/flaksp/anime365-sidecar/pkg/jikanclient"
)

func NewService(anime365Client *anime365client.Client, jikanClient *jikanclient.Client) *Service {
	return &Service{
		anime365Client: anime365Client,
		jikanClient:    jikanClient,
	}
}

type Service struct {
	anime365Client *anime365client.Client
	jikanClient    *jikanclient.Client
}

func (s *Service) GetEpisode(ctx context.Context, episodeID Anime365EpisodeID) (Episode, error) {
	episodeDTO, err := s.anime365Client.GetEpisode(ctx, int64(episodeID))
	if err != nil {
		return Episode{}, fmt.Errorf("getting episode from anime 365 api: %w", err)
	}

	episodeEntity, err := NewEpisode(episodeDTO)
	if err != nil {
		return Episode{}, fmt.Errorf("creating episode entity from anime 365 api dto: %w", err)
	}

	return episodeEntity, nil
}

func (s *Service) GetTranslation(
	ctx context.Context,
	translationID Anime365TranslationID,
) (Translation, error) {
	translationDTO, err := s.anime365Client.GetTranslation(ctx, int64(translationID))
	if err != nil {
		return Translation{}, fmt.Errorf("getting translation from anime 365 api: %w", err)
	}

	translationEntity, err := NewTranslation(translationDTO)
	if err != nil {
		return Translation{}, fmt.Errorf("creating translation from anime 365 api dto: %w", err)
	}

	return translationEntity, nil
}

func (s *Service) GetTranslationMedia(
	ctx context.Context,
	translationID Anime365TranslationID,
) (TranslationMedia, error) {
	translationEmbedDTO, err := s.anime365Client.GetTranslationEmbed(ctx, int64(translationID))
	if err != nil {
		return TranslationMedia{}, fmt.Errorf("getting translation embed from anime 365 api: %w", err)
	}

	translationMedia, err := NewTranslationMedia(translationEmbedDTO, s.anime365Client.BaseURL)
	if err != nil {
		return TranslationMedia{}, fmt.Errorf("creating translation media from anime 365 api dto: %w", err)
	}

	return translationMedia, nil
}

func (s *Service) MarkTranslationAsWatched(
	ctx context.Context,
	translationID Anime365TranslationID,
) error {
	err := s.anime365Client.MarkTranslationAsWatched(ctx, int64(translationID))
	if err != nil {
		return fmt.Errorf("marking translation as watched on anime 365: %w", err)
	}

	return nil
}

var ErrJikanEpisodeNotFound = errors.New("episode not found")

func (s *Service) GetEpisodeMetadataFromJikan(
	ctx context.Context,
	myAnimeListShowID int64,
	episodeNumber int64,
) (MetadataFromJikan, error) {
	episodeDTO, err := s.jikanClient.GetAnimeEpisodeByID(ctx, myAnimeListShowID, episodeNumber)
	if err != nil {
		if apiError, ok := errors.AsType[*jikanclient.APIError](
			err,
		); ok &&
			apiError.Status == jikanclient.ErrorCodeNotFound {
			return MetadataFromJikan{}, ErrJikanEpisodeNotFound
		}

		return MetadataFromJikan{}, fmt.Errorf("getting episode from jikan: %w", err)
	}

	if episodeDTO.Title == "" {
		return MetadataFromJikan{}, ErrJikanEpisodeNotFound
	}

	// TODO(flaksp): Remove strange logic by retrieving score below after the issue is fixed: https://github.com/jikan-me/jikan/issues/584

	// Jikan returns only 100 episodes per page, so we are basically guessing page by episode number
	page := episodeNumber / 100

	episodeListingItemDTOs, err := s.jikanClient.GetAnimeEpisodes(ctx, myAnimeListShowID, page)
	if err != nil {
		return MetadataFromJikan{}, err
	}

	var score float64

	for _, episodeListingItemDTO := range episodeListingItemDTOs {
		if episodeListingItemDTO.MalID != episodeNumber {
			continue
		}

		if episodeListingItemDTO.Score != nil {
			score = *episodeListingItemDTO.Score
		} else {
			score = 0
		}
	}

	return NewEpisodeMetadataFromJikan(episodeDTO, score)
}
