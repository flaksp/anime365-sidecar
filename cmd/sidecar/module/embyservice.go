package module

import (
	"log/slog"
	"path/filepath"

	"github.com/flaksp/anime365-emby/cmd/sidecar/config"
	"github.com/flaksp/anime365-emby/internal/emby"
	"github.com/flaksp/anime365-emby/pkg/embyclient"
)

var EmbyService = func(config *config.Env, logger *slog.Logger, embyClient *embyclient.Client) (*emby.Service, error) {
	downloadsDirectoryAbsolutePath, err := filepath.Abs(config.LibraryDirectory)
	if err != nil {
		return nil, err
	}

	return emby.NewService(
		downloadsDirectoryAbsolutePath,
		config.EmbyUserID,
		logger,
		embyClient,
	), nil
}
