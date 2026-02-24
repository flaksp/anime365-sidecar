package startup

import (
	"context"

	"github.com/flaksp/anime365-emby/cmd/sidecar/config"
	"github.com/flaksp/anime365-emby/internal/emby"
)

var DetectLibraryDirectoryFromEmby = func(config *config.Env, embyService *emby.Service) error {
	ctx := context.Background()

	return embyService.DetectLibraryDirectoryFromEmby(ctx, config.EmbyLibraryName)
}
