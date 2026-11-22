package worker

import (
	"log/slog"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/tenraimetadatarefresher"
	"github.com/flaksp/anime365-sidecar/pkg/backgroundworker"
	"go.uber.org/fx"
)

func TenraiMetadataRefresher(
	lc fx.Lifecycle,
	env *config.Env,
	logger *slog.Logger,
	refresher *tenraimetadatarefresher.Service,
) {
	worker := backgroundworker.New(
		"tenrai-metadata-refresher",
		env.MetadataRefreshIdleInterval,
		refresher.RunOnce,
		logger,
	)
	worker.Register(lc)
}
