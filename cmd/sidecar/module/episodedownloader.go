package module

import (
	"log/slog"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/episodedownloader"
	"github.com/flaksp/anime365-sidecar/internal/mediaserver"
	"github.com/flaksp/anime365-sidecar/internal/mylist"
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
	mediaServer mediaserver.Server,
	smartDownloader *downloader.SmartDownloader,
	anime365Client *anime365client.Client,
) (*episodedownloader.Service, error) {
	return episodedownloader.NewService(
		myListService,
		scanSource,
		episodeService,
		logger,
		mediaServer,
		smartDownloader,
		anime365Client,
		config.Translations,
		config.DownloadTimeoutVideo,
		config.TemporaryDirectory,
		config.PreferredTranslationAuthors,
		config.BlacklistedTranslationAuthors,
		config.EpisodesToDownloadAhead,
		config.WatchedEpisodesCleanupPolicy,
	), nil
}
