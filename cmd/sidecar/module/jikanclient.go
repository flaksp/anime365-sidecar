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
		5*time.Second,
		logger,
		rate.NewLimiter(rate.Limit(3), 3),
		rate.NewLimiter(rate.Every(time.Minute/60), 30),
	), nil
}
