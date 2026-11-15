package module

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/pkg/embyclient"
	"github.com/flaksp/anime365-sidecar/pkg/httproundtripperwithlogger"
)

var EmbyClient = func(config *config.Env, logger *slog.Logger) (*embyclient.ClientWithResponses, error) {
	return embyclient.NewClientWithResponses(
		config.EmbyBaseURL.String(),
		embyclient.WithHTTPClient(&http.Client{
			Transport: httproundtripperwithlogger.New(http.DefaultTransport, logger),
			Timeout:   5 * time.Second,
		}),
		embyclient.WithRequestEditorFn(func(_ context.Context, request *http.Request) error {
			request.Header.Set("Accept", "application/json")
			request.Header.Set("X-Emby-Token", config.EmbyAPIKey)
			// Emby decodes query parameters as URL paths, so spaces must use %20.
			request.URL.RawQuery = strings.ReplaceAll(request.URL.RawQuery, "+", "%20")

			return nil
		}),
	)
}
