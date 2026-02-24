package httproundtripperwithlogger

import (
	"log/slog"
	"net/http"
)

type HTTPRoundTripperWithLogger struct {
	baseRoundTripper http.RoundTripper
	logger           *slog.Logger
}

func New(baseRoundTripper http.RoundTripper, logger *slog.Logger) *HTTPRoundTripperWithLogger {
	if baseRoundTripper == nil {
		baseRoundTripper = http.DefaultTransport
	}

	return &HTTPRoundTripperWithLogger{
		baseRoundTripper: baseRoundTripper,
		logger:           logger,
	}
}

func (t *HTTPRoundTripperWithLogger) RoundTrip(httpRequest *http.Request) (*http.Response, error) {
	httpResponse, err := t.baseRoundTripper.RoundTrip(httpRequest)
	if err != nil {
		return nil, err
	}

	t.logger.DebugContext(
		httpRequest.Context(),
		"HTTP request",
		slog.String("method", httpRequest.Method),
		slog.String("url", httpRequest.URL.String()),
		slog.String("status", httpResponse.Status),
	)

	return httpResponse, nil
}
