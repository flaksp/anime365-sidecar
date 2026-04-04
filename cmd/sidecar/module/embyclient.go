package module

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/pkg/embyclient"
	"github.com/flaksp/anime365-sidecar/pkg/httproundtripperwithlogger"
)

var EmbyClient = func(config *config.Env, logger *slog.Logger) (*embyclient.Client, error) {
	return embyclient.New(
		config.EmbyBaseURL,
		&http.Client{
			Transport: httproundtripperwithlogger.New(http.DefaultTransport, logger),
		},
		5*time.Second,
		logger,
		config.EmbyAPIKey,
	), nil
}
