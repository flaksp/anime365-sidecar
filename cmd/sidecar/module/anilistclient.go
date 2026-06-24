package module

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/pkg/anilistclient"
	"github.com/flaksp/anime365-sidecar/pkg/httproundtripperwithlogger"
	"golang.org/x/time/rate"
)

var AnilistClient = func(config *config.Env, logger *slog.Logger) (*anilistclient.Client, error) {
	return anilistclient.New(
		config.AnilistAPIBaseURL,
		&http.Client{
			Transport: httproundtripperwithlogger.New(http.DefaultTransport, logger),
		},
		10*time.Second,
		logger,
		rate.NewLimiter(
			rate.Every(time.Minute/20),
			2,
		), //  Actual limit is 30 RPM, but we are limiting to 20 RPM + burst of 2
	), nil
}
