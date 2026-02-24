package module

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/flaksp/anime365-emby/cmd/sidecar/config"
	"github.com/flaksp/anime365-emby/pkg/httproundtripperwithlogger"
	"github.com/flaksp/anime365-emby/pkg/shikimoriclient"
)

var ShikimoriClient = func(config *config.Env, logger *slog.Logger) (*shikimoriclient.Client, error) {
	return shikimoriclient.New(
		config.ShikimoriBaseURL,
		&http.Client{
			Transport: httproundtripperwithlogger.New(http.DefaultTransport, logger),
		},
		5*time.Second,
		logger,
	), nil
}
