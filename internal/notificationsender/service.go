package notificationsender

import (
	"context"
	"fmt"
	"net/url"

	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/pkg/authorslistformatter"
	"github.com/flaksp/anime365-sidecar/pkg/filesize"
	"github.com/flaksp/anime365-sidecar/pkg/telegrambotapiclient"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
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
	embyWebURL *url.URL,
	showName string,
	episodeLabel string,
	translationVariant episode.TranslationVariant,
	authorsList []string,
	videoMetadataDisplayTitle string,
	fileBitrate int64,
) error {
	if s.telegramBotAPIClient == nil {
		return nil
	}

	_, err := s.telegramBotAPIClient.SendMessage(
		ctx,
		s.telegramRecipient,
		fmt.Sprintf(
			"💾 Загружено: <a href=\"%s\">%s</a>, %s. %s %s by %s (%s, %s)",
			embyWebURL.String(),
			episodeLabel,
			showName,
			display.English.Languages().Name(translationVariant.Language),
			cases.Title(language.English).String(translationVariant.Kind.Label()),
			authorslistformatter.Format(authorsList),
			videoMetadataDisplayTitle,
			filesize.FormatBitrate(fileBitrate),
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
