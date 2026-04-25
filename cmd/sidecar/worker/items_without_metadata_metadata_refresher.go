package worker

import (
	"context"
	"log/slog"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/librarymetadatarefresher"
	"github.com/flaksp/anime365-sidecar/pkg/backgroundworker"
	"go.uber.org/fx"
)

var ItemsWithoutMetadataMetadataRefresher = func(
	lc fx.Lifecycle,
	config *config.Env,
	logger *slog.Logger,
	libraryMetadataRefresher *librarymetadatarefresher.Service,
) error {
	worker := backgroundworker.New(
		"items-without-metadata-metadata-refresher",
		config.FreshItemsMetadataRefreshIdleInterval,
		func(ctx context.Context) error {
			return libraryMetadataRefresher.RunOnceForItemsWithoutMetadata(ctx)
		},
		logger,
	)

	worker.Register(lc)

	return nil
}
