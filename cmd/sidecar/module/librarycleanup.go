package module

import (
	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/internal/librarycleanup"
)

var LibraryCleanupOptions = func(config *config.Env) librarycleanup.Options {
	return librarycleanup.Options{
		Policy:                    config.WatchedEpisodesCleanupPolicy,
		DeleteAfter:               config.WatchedEpisodesCleanupAfter,
		DeleteRemovedTranslations: config.DeleteRemovedTranslations,
	}
}
