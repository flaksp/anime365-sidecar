package module

import (
	"log/slog"
	"net/http"

	"github.com/flaksp/anime365-sidecar/pkg/downloader"
)

var SimpleDownloader = func(logger *slog.Logger) *downloader.SimpleDownloader {
	return downloader.NewSimpleDownloader(
		&http.Client{},
		logger,
	)
}
