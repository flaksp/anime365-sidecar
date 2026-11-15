package module

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/pkg/httproundtripperwithlogger"
	"github.com/flaksp/anime365-sidecar/pkg/jellyfinclient"
)

var JellyfinClient = func(config *config.Env, logger *slog.Logger) (*jellyfinclient.ClientWithResponses, error) {
	return jellyfinclient.NewClientWithResponses(
		config.JellyfinBaseURL.String(),
		jellyfinclient.WithHTTPClient(
			&http.Client{
				Transport: httproundtripperwithlogger.New(http.DefaultTransport, logger),
				Timeout:   5 * time.Second,
			},
		),
		jellyfinclient.WithRequestEditorFn(func(_ context.Context, request *http.Request) error {
			request.Header.Set("Accept", "application/json")
			request.Header.Set(
				"Authorization",
				`MediaBrowser Client="Anime365 Sidecar", Device="Sidecar", DeviceId="anime365-sidecar", Version="1", Token="`+config.JellyfinAPIKey+`"`,
			)

			return nil
		}),
	)
}
