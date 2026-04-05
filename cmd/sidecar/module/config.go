package module

import (
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
		err := godotenv.Load()
		if err != nil {
			return nil, err
		}
	}

	var config config.Env

	err := env.Parse(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
