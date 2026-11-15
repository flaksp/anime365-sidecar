package module

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/animemapping"
	"github.com/flaksp/anime365-sidecar/internal/emby"
	"github.com/flaksp/anime365-sidecar/internal/jellyfin"
	"github.com/flaksp/anime365-sidecar/internal/mediaserver"
	"github.com/google/uuid"
)

var MediaServer = func(config *config.Env, logger *slog.Logger, animeMappingService *animemapping.Service) (mediaserver.Server, error) {
	directory, err := filepath.Abs(config.LibraryDirectory)
	if err != nil {
		return nil, err
	}

	switch config.MediaServer {
	case "emby":
		client, err := EmbyClient(config, logger)
		if err != nil {
			return nil, err
		}

		publicURL := config.EmbyBaseURL
		if config.EmbyPublicBaseURL != nil {
			publicURL = config.EmbyPublicBaseURL
		}

		return emby.NewService(
			directory,
			config.EmbyUserID,
			config.EmbyLibraryID,
			logger,
			client,
			publicURL,
			animeMappingService,
		), nil
	case "jellyfin":
		client, err := JellyfinClient(config, logger)
		if err != nil {
			return nil, err
		}

		libraryID, err := uuid.Parse(config.JellyfinLibraryID)
		if err != nil {
			return nil, fmt.Errorf("parse Jellyfin library ID: %w", err)
		}

		userID, err := uuid.Parse(config.JellyfinUserID)
		if err != nil {
			return nil, fmt.Errorf("parse Jellyfin user ID: %w", err)
		}

		publicURL := config.JellyfinBaseURL
		if config.JellyfinPublicBaseURL != nil {
			publicURL = config.JellyfinPublicBaseURL
		}

		return jellyfin.NewService(directory, userID, libraryID, logger, client, publicURL, animeMappingService), nil
	default:
		return nil, fmt.Errorf("unsupported media server %q", config.MediaServer)
	}
}
