package module

import (
	"log/slog"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/librarymetadatarefresher"
	"github.com/flaksp/anime365-sidecar/internal/mediaserver"
	"github.com/flaksp/anime365-sidecar/internal/notificationsender"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/downloader"
)

var LibraryMetadataRefresher = func(
	config *config.Env,
	showService *show.Service,
	episodeService *episode.Service,
	mediaServer mediaserver.Server,
	simpleDownloader *downloader.SimpleDownloader,
	logger *slog.Logger,
	notificationSenderService *notificationsender.Service,
) (*librarymetadatarefresher.Service, error) {
	return librarymetadatarefresher.NewService(
		showService,
		episodeService,
		mediaServer,
		simpleDownloader,
		logger,
		notificationSenderService,
		config.DownloadTimeoutImage,
		config.TemporaryDirectory,
	), nil
}
