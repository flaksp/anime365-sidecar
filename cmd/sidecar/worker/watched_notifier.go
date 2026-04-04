package worker

import (
	"context"
	"log/slog"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/watchednotifier"
	"github.com/flaksp/anime365-sidecar/pkg/backgroundworker"
	"go.uber.org/fx"
)

var WatchedNotifier = func(
	lc fx.Lifecycle,
	config *config.Env,
	logger *slog.Logger,
	watchedNotifier *watchednotifier.Service,
) error {
	worker := backgroundworker.New(
		"watched-notifier",
		config.ScanIdleInterval,
		func(ctx context.Context) error {
			return watchedNotifier.RunOnce(ctx)
		},
		logger,
	)

	worker.Register(lc)

	return nil
}
