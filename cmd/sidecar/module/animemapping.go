package module

import (
	"log/slog"
	"net/http"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/animemapping"
	"github.com/flaksp/anime365-sidecar/pkg/httproundtripperwithlogger"
)

var AnimeMappingService = func(config *config.Env, logger *slog.Logger) (*animemapping.Service, error) {
	httpClient := &http.Client{
		Transport: httproundtripperwithlogger.New(nil, logger),
	}

	return animemapping.NewService(
		config.AnimeListDatabaseURL,
		httpClient,
		logger,
	)
}
