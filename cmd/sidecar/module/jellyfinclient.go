package module

import (
	"context"
	"log/slog"
	"mime"
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
				Transport: jellyfinTransport{base: httproundtripperwithlogger.New(http.DefaultTransport, logger)},
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

// Jellyfin declares JSON profile variants in OpenAPI, so the generated parser
// compares Content-Type literally. Real responses also include charset parameters.
// All declared profiles use the same models; let the parser use its JSON branch.
type jellyfinTransport struct {
	base http.RoundTripper
}

func (t jellyfinTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}

	mediaType, _, parseErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if parseErr == nil && mediaType == "application/json" {
		response.Header.Set("Content-Type", mediaType)
	}

	return response, nil
}
