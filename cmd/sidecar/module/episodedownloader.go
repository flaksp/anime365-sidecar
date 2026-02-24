package module

import (
	"log/slog"

	"github.com/flaksp/anime365-emby/cmd/sidecar/config"
	"github.com/flaksp/anime365-emby/internal/emby"
	"github.com/flaksp/anime365-emby/internal/episode"
	"github.com/flaksp/anime365-emby/internal/episodedownloader"
	"github.com/flaksp/anime365-emby/internal/mylist"
	"github.com/flaksp/anime365-emby/internal/scansource"
	"github.com/flaksp/anime365-emby/pkg/anime365client"
	"github.com/flaksp/anime365-emby/pkg/downloader"
)

var EpisodeDownloader = func(
	config *config.Env,
	myListService *mylist.Service,
	scanSource *scansource.Service,
	episodeService *episode.Service,
	logger *slog.Logger,
	embyService *emby.Service,
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
		config.Translations,
	), nil
}
