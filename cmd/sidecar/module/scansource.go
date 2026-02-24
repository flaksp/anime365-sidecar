package module

import (
	"github.com/flaksp/anime365-emby/cmd/sidecar/config"
	"github.com/flaksp/anime365-emby/internal/scansource"
)

var ScanSource = func(
	config *config.Env,
) (*scansource.Service, error) {
	return scansource.NewService(
		config.ScanSources,
	), nil
}
