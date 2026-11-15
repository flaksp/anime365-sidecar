package module

import (
	"log/slog"
	"path/filepath"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/animemapping"
	"github.com/flaksp/anime365-sidecar/internal/emby"
	"github.com/flaksp/anime365-sidecar/internal/mediaserver"
	"github.com/flaksp/anime365-sidecar/pkg/embyclient"
)

var MediaServer = func(
	config *config.Env,
	logger *slog.Logger,
	embyClient *embyclient.ClientWithResponses,
	animeMappingService *animemapping.Service,
) (mediaserver.Server, error) {
	downloadsDirectoryAbsolutePath, err := filepath.Abs(config.LibraryDirectory)
	if err != nil {
		return nil, err
	}

	embyPublicURL := config.EmbyBaseURL

	if config.EmbyPublicBaseURL != nil {
		embyPublicURL = config.EmbyPublicBaseURL
	}

	return emby.NewService(
		downloadsDirectoryAbsolutePath,
		config.EmbyUserID,
		config.EmbyLibraryID,
		logger,
		embyClient,
		embyPublicURL,
		animeMappingService,
	), nil
}
