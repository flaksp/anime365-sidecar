package module

import (
	"log/slog"
	"os"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
)

var Logger = func(config *config.Env) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.LogLevel,
	}))
}
