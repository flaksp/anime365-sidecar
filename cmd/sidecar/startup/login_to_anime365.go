package startup

import (
	"context"
	"log/slog"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/mylist"
	"github.com/flaksp/anime365-sidecar/pkg/anime365client"
)

var LoginToAnime365 = func(config *config.Env, logger *slog.Logger, client *anime365client.Client, myListService *mylist.Service) error {
	ctx := context.Background()

	err := client.Login(
		ctx,
		config.Anime365Login,
		config.Anime365Password,
	)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to login", slog.String("error", err.Error()))

		return err
	}

	profile, err := client.GetMe(
		ctx,
	)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get current user", slog.String("error", err.Error()))

		return err
	}

	logger.InfoContext(
		ctx,
		"Successfully logged in to Anime 365",
		slog.String("profile_name", profile.Name),
		slog.Int64("profile_id", profile.ID),
	)

	return nil
}
