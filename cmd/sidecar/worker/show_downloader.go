package worker

import (
	"context"
	"log/slog"

	"github.com/flaksp/anime365-emby/cmd/sidecar/config"
	"github.com/flaksp/anime365-emby/internal/showdownloader"
	"github.com/flaksp/anime365-emby/pkg/backgroundworker"
	"go.uber.org/fx"
)

var ShowDownloader = func(
	lc fx.Lifecycle,
	logger *slog.Logger,
	config *config.Env,
	showDownloader *showdownloader.Service,
) error {
	worker := backgroundworker.New(
		"show-downloader",
		config.ScanIdleInterval,
		func(ctx context.Context) error {
			return showDownloader.RunOnce(ctx)
		},
		logger,
	)

	worker.Register(lc)

	return nil
}
