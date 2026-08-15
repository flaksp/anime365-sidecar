package backgroundworker

import (
	"context"
	"log/slog"
	"time"

	"go.uber.org/fx"
)

type Job func(ctx context.Context) error

type Option func(worker *Worker)

type Worker struct {
	job          Job
	logger       *slog.Logger
	name         string
	interval     time.Duration
	skipFirstRun bool
}

func WithSkipFirstRun() Option {
	return func(worker *Worker) {
		worker.skipFirstRun = true
	}
}

func New(
	name string,
	interval time.Duration,
	job Job,
	logger *slog.Logger,
	options ...Option,
) *Worker {
	worker := &Worker{
		name:     name,
		interval: interval,
		job:      job,
		logger:   logger,
	}

	for _, option := range options {
		option(worker)
	}

	return worker
}

func (w *Worker) Register(lc fx.Lifecycle) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			w.logger.InfoContext(ctx, "Starting background worker",
				slog.String("name", w.name),
				slog.Duration("interval", w.interval),
			)

			go w.loop(context.Background()) // nolint:contextcheck

			return nil
		},
		OnStop: func(ctx context.Context) error {
			w.logger.InfoContext(ctx, "Stopping background worker", slog.String("name", w.name))

			return nil
		},
	})
}

func (w *Worker) loop(ctx context.Context) {
	if w.skipFirstRun && !w.waitForNextRun(ctx) {
		return
	}

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

		if !w.waitForNextRun(ctx) {
			return
		}
	}
}

func (w *Worker) waitForNextRun(ctx context.Context) bool {
	timer := time.NewTimer(w.interval)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		w.logger.InfoContext(ctx, "Worker context cancelled", slog.String("name", w.name))

		return false
	}
}
