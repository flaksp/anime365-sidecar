package module

import (
	"errors"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/joho/godotenv"
)

func fileExists(absolutePath string) bool {
	info, err := os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}

		return false
	}

	if info.IsDir() {
		return false
	}

	return true
}

var Config = func() (*config.Env, error) {
	if fileExists(".env") {
		if err := godotenv.Load(); err != nil {
			return nil, err
		}
	}

	var config config.Env

	if err := env.Parse(&config); err != nil {
		return nil, err
	}

	if config.WatchedEpisodesCleanupAfter < 0 {
		return nil, errors.New("SIDECAR_WATCHED_EPISODES_CLEANUP_AFTER must not be negative")
	}

	return &config, nil
}
