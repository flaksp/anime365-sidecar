package worker

import (
	"context"
	"log/slog"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/animemapping"
	"github.com/flaksp/anime365-sidecar/pkg/backgroundworker"
	"go.uber.org/fx"
)

var AnimeMappingDatabaseRefresher = func(
	lc fx.Lifecycle,
	config *config.Env,
	logger *slog.Logger,
	animeMappingService *animemapping.Service,
) error {
	worker := backgroundworker.New(
		"anime-mapping-database-refresher",
		config.AnimeListRefreshIdleInterval,
		func(ctx context.Context) error {
			return animeMappingService.Refresh(ctx)
		},
		logger,
		backgroundworker.WithSkipFirstRun(),
	)

	worker.Register(lc)

	return nil
}
