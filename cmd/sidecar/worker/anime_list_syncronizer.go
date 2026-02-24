package worker

import (
	"context"
	"log/slog"

	"github.com/flaksp/anime365-emby/cmd/sidecar/config"
	"github.com/flaksp/anime365-emby/internal/mylist"
	"github.com/flaksp/anime365-emby/pkg/anime365client"
	"github.com/flaksp/anime365-emby/pkg/backgroundworker"
	"go.uber.org/fx"
)

var AnimeListSyncronizer = func(
	lc fx.Lifecycle,
	logger *slog.Logger,
	config *config.Env,
	anime365Client *anime365client.Client,
	myListService *mylist.Service,
) error {
	profile, err := anime365Client.GetMe(context.Background())
	if err != nil {
		return err
	}

	worker := backgroundworker.New(
		"anime-list-syncronizer",
		config.ScanIdleInterval,
		func(ctx context.Context) error {
			return myListService.LoadFromAnime365(ctx, profile.ID)
		},
		logger,
	)

	worker.Register(lc)

	return nil
}
