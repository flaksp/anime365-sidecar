package main

import (
	"log/slog"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/module"
	"github.com/flaksp/anime365-sidecar/cmd/sidecar/startup"
	"github.com/flaksp/anime365-sidecar/cmd/sidecar/worker"
	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/mylist"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/internal/showdownloader"
	"github.com/flaksp/anime365-sidecar/internal/watchednotifier"
	"github.com/flaksp/anime365-sidecar/pkg/downloader"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

func main() {
	fx.New(
		fx.Provide(module.Config),
		fx.Provide(module.Logger),
		fx.WithLogger(func(logger *slog.Logger) fxevent.Logger {
			return &fxevent.SlogLogger{Logger: logger}
		}),
		fx.Provide(module.Anime365Client),
		fx.Provide(module.EmbyClient),
		fx.Provide(module.ShikimoriClient),
		fx.Provide(module.JikanClient),
		fx.Provide(module.NotificationSender),
		fx.Provide(module.TelegramBotAPIClient),
		fx.Provide(module.EmbyService),
		fx.Provide(showdownloader.NewService),
		fx.Provide(module.EpisodeDownloader),
		fx.Provide(module.ScanSource),
		fx.Provide(module.LibraryMetadataRefresher),
		fx.Provide(watchednotifier.NewService),
		fx.Provide(show.NewService),
		fx.Provide(episode.NewService),
		fx.Provide(mylist.NewService),
		fx.Provide(module.SimpleDownloader),
		fx.Provide(module.ChunkedDownloader),
		fx.Provide(downloader.NewSmartDownloader),
		fx.Invoke(startup.DetectLibraryDirectoryFromEmby),
		fx.Invoke(startup.LoadEmbyLibraryManifestFromDisk),
		fx.Invoke(startup.LoginToAnime365),
		fx.Invoke(startup.LoadListFromAnime365),
		fx.Invoke(worker.AnimeListSyncronizer),
		fx.Invoke(worker.ShowDownloader),
		fx.Invoke(worker.LibraryMetadataRefresher),
		fx.Invoke(worker.WatchedNotifier),
	).Run()
}
