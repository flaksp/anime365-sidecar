package module

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/pkg/httproundtripperwithlogger"
	"github.com/flaksp/anime365-sidecar/pkg/shikimoriclient"
	"golang.org/x/time/rate"
)

var ShikimoriClient = func(config *config.Env, logger *slog.Logger) (*shikimoriclient.Client, error) {
	return shikimoriclient.New(
		config.ShikimoriBaseURL,
		&http.Client{
			Transport: httproundtripperwithlogger.New(http.DefaultTransport, logger),
		},
		5*time.Second,
		logger,
		rate.NewLimiter(rate.Limit(4), 4), //  Actual limit is 5 RPS, but we are limiting to 4 RPS + burst of 4
		rate.NewLimiter(
			rate.Every(time.Minute/90),
			4,
		), //  Actual limit is 90 RPM, but we are limiting to 90 RPM + burst of 4
	), nil
}
