package module

import (
	"errors"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/notificationsender"
	"github.com/flaksp/anime365-sidecar/pkg/telegrambotapiclient"
)

var NotificationSender = func(config *config.Env, telegramBotAPIClient *telegrambotapiclient.Client) (*notificationsender.Service, error) {
	if config.TelegramBotAPICredentials == nil {
		return nil, nil // nolint:nilnil
	}

	if telegramBotAPIClient == nil {
		return nil, nil // nolint:nilnil
	}

	recipient := config.TelegramBotAPICredentials.Query().Get("recipient")

	if recipient == "" {
		return nil, errors.New("invalid telegram config - missing recipient param")
	}

	return notificationsender.NewService(
		telegramBotAPIClient,
		recipient,
	), nil
}
