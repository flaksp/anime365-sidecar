package startup

import (
	"context"

	"github.com/flaksp/anime365-sidecar/internal/mediaserver"
)

var InitializeMediaServer = func(server mediaserver.Server) error {
	return server.Initialize(context.Background())
}
