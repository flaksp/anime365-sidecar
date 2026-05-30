package module

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/flaksp/anime365-sidecar/pkg/downloader"
)

var ChunkedDownloader = func(logger *slog.Logger) *downloader.ChunkedDownloader {
	return downloader.NewChunkedDownloader(
		&http.Client{
			Transport: &http.Transport{
				ForceAttemptHTTP2: false,
			},
		},
		logger,
		1024*1024*16,
		4,
		3,
		time.Second,
	)
}
