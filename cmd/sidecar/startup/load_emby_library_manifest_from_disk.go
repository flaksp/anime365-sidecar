package startup

import (
	"context"

	"github.com/flaksp/anime365-sidecar/internal/emby"
)

var LoadEmbyLibraryManifestFromDisk = func(embyService *emby.Service) error {
	ctx := context.Background()

	return embyService.LoadManifestFromDisk(ctx)
}
