package module

import (
	"log/slog"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/emby"
	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/librarymetadatarefresher"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/downloader"
)

var LibraryMetadataRefresher = func(
	config *config.Env,
	showService *show.Service,
	episodeService *episode.Service,
	embyService *emby.Service,
	smartDownloader *downloader.SmartDownloader,
	logger *slog.Logger,
) (*librarymetadatarefresher.Service, error) {
	return librarymetadatarefresher.NewService(
		showService,
		episodeService,
		embyService,
		smartDownloader,
		logger,
		config.DownloadTimeoutImage,
	), nil
}
