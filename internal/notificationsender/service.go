package notificationsender

import (
	"context"
	"fmt"
	"strings"

	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/telegrambotapiclient"
)

func NewService(telegramBotAPIClient *telegrambotapiclient.Client, telegramRecipient string) *Service {
	return &Service{
		telegramBotAPIClient: telegramBotAPIClient,
		telegramRecipient:    telegramRecipient,
	}
}

type Service struct {
	telegramBotAPIClient *telegrambotapiclient.Client
	telegramRecipient    string
}

func (s *Service) TranslationDownloaded(
	ctx context.Context,
	showEntity show.Show,
	episodeEntity episode.Episode,
	translationEntity episode.Translation,
	translationMediaEntity episode.TranslationMedia,
) error {
	if s.telegramBotAPIClient == nil {
		return nil
	}

	_, err := s.telegramBotAPIClient.SendMessage(
		ctx,
		s.telegramRecipient,
		fmt.Sprintf(
			"💾 <a href=\"%s\">%s</a>: загружена <b>%s</b> в переводе <a href=\"%s\">%s</a> (%dp)",
			showEntity.Anime365URL.String(),
			showEntity.TitleRussian,
			episodeEntity.EpisodeLabel,
			translationEntity.Anime365URL.String(),
			strings.Join(translationEntity.Authors, ", "),
			translationMediaEntity.Height,
		),
		&telegrambotapiclient.SendMessageOptionalParams{
			ParseMode:          "HTML",
			LinkPreviewOptions: &telegrambotapiclient.LinkPreviewOptions{IsDisabled: new(true)},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to send telegram bot message: %w", err)
	}

	return nil
}
