package backgroundworker

import (
	"context"
	"log/slog"
	"time"

	"go.uber.org/fx"
)

type Job func(ctx context.Context) error

type Worker struct {
	job      Job
	logger   *slog.Logger
	name     string
	interval time.Duration
}

func New(
	name string,
	interval time.Duration,
	job Job,
	logger *slog.Logger,
) *Worker {
	return &Worker{
		name:     name,
		interval: interval,
		job:      job,
		logger:   logger,
	}
}

func (w *Worker) Register(lc fx.Lifecycle) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			w.logger.InfoContext(ctx, "Starting background worker",
				slog.String("name", w.name),
				slog.Duration("interval", w.interval),
			)

			go w.loop(context.Background())

			return nil
		},
		OnStop: func(ctx context.Context) error {
			w.logger.InfoContext(ctx, "Stopping background worker", slog.String("name", w.name))

			return nil
		},
	})
}

func (w *Worker) loop(ctx context.Context) {
	for {
		w.logger.InfoContext(ctx, "Worker started", slog.String("name", w.name))

		if err := w.job(ctx); err != nil {
			w.logger.ErrorContext(ctx, "Worker failed",
				slog.String("name", w.name),
				slog.String("error", err.Error()),
			)
		} else {
			w.logger.InfoContext(ctx, "Worker finished", slog.String("name", w.name))
		}

		select {
		case <-time.After(w.interval):
			continue
		case <-ctx.Done():
			w.logger.InfoContext(ctx, "Worker context cancelled", slog.String("name", w.name))

			return
		}
	}
}
