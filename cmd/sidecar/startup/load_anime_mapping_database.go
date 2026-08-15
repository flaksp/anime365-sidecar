package startup

import (
	"context"
	"log/slog"

	"github.com/flaksp/anime365-sidecar/internal/animemapping"
)

var LoadAnimeMappingDatabase = func(logger *slog.Logger, animeMappingService *animemapping.Service) error {
	ctx := context.Background()

	if err := animeMappingService.Refresh(ctx); err != nil {
		logger.WarnContext(
			ctx,
			"Failed to load anime mapping database, continuing with an empty in-memory database",
			slog.String("error", err.Error()),
		)
	}

	return nil
}
