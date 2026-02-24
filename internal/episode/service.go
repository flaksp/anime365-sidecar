package episode

import (
	"context"
	"fmt"

	"github.com/flaksp/anime365-emby/pkg/anime365client"
)

func NewService(anime365Client *anime365client.Client) *Service {
	return &Service{
		anime365Client: anime365Client,
	}
}

type Service struct {
	anime365Client *anime365client.Client
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
