package module

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/pkg/httproundtripperwithlogger"
	"github.com/flaksp/anime365-sidecar/pkg/tenraiclient"
	"golang.org/x/time/rate"
)

var TenraiClient = func(config *config.Env, logger *slog.Logger) (*tenraiclient.ClientWithResponses, error) {
	// Leave room under the public 120 RPM limit.
	limiter := rate.NewLimiter(rate.Every(time.Minute/100), 1)

	return tenraiclient.NewClientWithResponses(
		config.TenraiAPIBaseURL.String(),
		tenraiclient.WithHTTPClient(&http.Client{
			Transport: httproundtripperwithlogger.New(http.DefaultTransport, logger),
			Timeout:   10 * time.Second,
		}),
		tenraiclient.WithRequestEditorFn(func(ctx context.Context, request *http.Request) error {
			request.Header.Set("Accept", "application/json")

			return limiter.Wait(ctx)
		}),
	)
}
