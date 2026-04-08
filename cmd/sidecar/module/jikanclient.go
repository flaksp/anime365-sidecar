package module

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/pkg/httproundtripperwithlogger"
	"github.com/flaksp/anime365-sidecar/pkg/jikanclient"
	"golang.org/x/time/rate"
)

var JikanClient = func(config *config.Env, logger *slog.Logger) (*jikanclient.Client, error) {
	return jikanclient.New(
		config.JikanAPIBaseURL,
		&http.Client{
			Transport: httproundtripperwithlogger.New(http.DefaultTransport, logger),
		},
		10*time.Second,
		logger,
		rate.NewLimiter(rate.Limit(2), 2), //  Actual limit is 3 RPS, but we are limiting to 2 RPS + burst of 2
		rate.NewLimiter(
			rate.Every(time.Minute/40),
			3,
		), //  Actual limit is 60 RPM, but we are limiting to 40 RPM + burst of 3
	), nil
}
