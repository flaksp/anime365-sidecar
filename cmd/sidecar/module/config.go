package module

import (
	"github.com/caarlos0/env/v11"
	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/joho/godotenv"
)

var Config = func() (*config.Env, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	var config config.Env

	err = env.Parse(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
