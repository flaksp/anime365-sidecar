package module

import (
	"log/slog"
	"net/http"

	"github.com/flaksp/anime365-emby/pkg/downloader"
)

var ChunkedDownloader = func(logger *slog.Logger) *downloader.ChunkedDownloader {
	return downloader.NewChunkedDownloader(
		&http.Client{},
		logger,
		4<<20,
		8,
	)
}
