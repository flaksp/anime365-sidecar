package module

import (
	"log/slog"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/emby"
	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/episodedownloader"
	"github.com/flaksp/anime365-sidecar/internal/mylist"
	"github.com/flaksp/anime365-sidecar/internal/notificationsender"
	"github.com/flaksp/anime365-sidecar/internal/scansource"
	"github.com/flaksp/anime365-sidecar/pkg/anime365client"
	"github.com/flaksp/anime365-sidecar/pkg/downloader"
)

var EpisodeDownloader = func(
	config *config.Env,
	myListService *mylist.Service,
	scanSource *scansource.Service,
	episodeService *episode.Service,
	logger *slog.Logger,
	embyService *emby.Service,
	notificationSenderService *notificationsender.Service,
	smartDownloader *downloader.SmartDownloader,
	anime365Client *anime365client.Client,
) (*episodedownloader.Service, error) {
	return episodedownloader.NewService(
		myListService,
		scanSource,
		episodeService,
		logger,
		embyService,
		smartDownloader,
		anime365Client,
		notificationSenderService,
		config.Translations,
		config.DownloadTimeoutVideo,
	), nil
}
