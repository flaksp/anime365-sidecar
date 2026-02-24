package startup

import (
	"context"

	"github.com/flaksp/anime365-emby/internal/emby"
)

var LoadEmbyLibraryManifestFromDisk = func(embyService *emby.Service) error {
	ctx := context.Background()

	return embyService.LoadManifestFromDisk(ctx)
}
