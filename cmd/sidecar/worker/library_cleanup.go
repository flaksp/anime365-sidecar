package worker

import (
	"log/slog"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/librarycleanup"
	"github.com/flaksp/anime365-sidecar/pkg/backgroundworker"
	"go.uber.org/fx"
)

var LibraryCleanup = func(
	lc fx.Lifecycle,
	logger *slog.Logger,
	config *config.Env,
	libraryCleanup *librarycleanup.Service,
) error {
	if !config.DeleteRemovedTranslations {
		return nil
	}

	worker := backgroundworker.New(
		"library-cleanup",
		config.ScanIdleInterval,
		libraryCleanup.RunOnce,
		logger,
	)

	worker.Register(lc)

	return nil
}
