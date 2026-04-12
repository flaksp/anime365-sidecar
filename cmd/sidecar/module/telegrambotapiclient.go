package module

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/pkg/httproundtripperwithlogger"
	"github.com/flaksp/anime365-sidecar/pkg/telegrambotapiclient"
)

var TelegramBotAPIClient = func(config *config.Env, logger *slog.Logger) (*telegrambotapiclient.Client, error) {
	if config.TelegramBotAPICredentials == nil {
		return nil, nil // nolint:nilnil
	}

	baseURL, err := url.Parse("https://" + config.TelegramBotAPICredentials.Host)
	if err != nil {
		return nil, fmt.Errorf(
			"invalid telegram config - failed to parse telegram bot api url, probably host is invalid: %w",
			err,
		)
	}

	if config.TelegramBotAPICredentials.User == nil {
		return nil, errors.New("invalid telegram config - missing telegram bot token")
	}

	return telegrambotapiclient.New(
		baseURL,
		config.TelegramBotAPICredentials.User.String(),
		&http.Client{
			Transport: httproundtripperwithlogger.New(http.DefaultTransport, logger),
		},
		10*time.Second,
		logger,
	), nil
}
