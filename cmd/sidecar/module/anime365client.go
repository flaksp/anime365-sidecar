package module

import (
	"log"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"time"

	"github.com/flaksp/anime365-sidecar/cmd/sidecar/config"
	"github.com/flaksp/anime365-sidecar/pkg/anime365client"
	"github.com/flaksp/anime365-sidecar/pkg/httproundtripperwithlogger"
)

var Anime365Client = func(config *config.Env, logger *slog.Logger) (*anime365client.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatal(err)
	}

	return anime365client.New(
		config.Anime365BaseURL,
		&http.Client{
			Transport: httproundtripperwithlogger.New(http.DefaultTransport, logger),
			Jar:       jar,
		},
		5*time.Second,
		logger,
	), nil
}
