package worker

import (
	"context"
	"log/slog"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/librarymetadatarefresher"
	"github.com/flaksp/anime365-sidecar/pkg/backgroundworker"
	"go.uber.org/fx"
)

var LibraryMetadataRefresher = func(
	lc fx.Lifecycle,
	config *config.Env,
	logger *slog.Logger,
	libraryMetadataRefresher *librarymetadatarefresher.Service,
) error {
	worker := backgroundworker.New(
		"library-metadata-refresher",
		config.MetadataRefreshIdleInterval,
		func(ctx context.Context) error {
			return libraryMetadataRefresher.RunOnce(ctx)
		},
		logger,
	)

	worker.Register(lc)

	return nil
}
