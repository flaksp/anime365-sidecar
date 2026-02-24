package config

import (
	"net/url"
	"time"
)

type Env struct {
	Anime365BaseURL               *url.URL      `env:"SIDECAR_ANIME365_BASE_URL,required,notEmpty"`
	EmbyBaseURL                   *url.URL      `env:"SIDECAR_EMBY_BASE_URL,required,notEmpty"`
	ShikimoriBaseURL              *url.URL      `env:"SIDECAR_SHIKIMORI_BASE_URL,required,notEmpty"`
	Anime365Login                 string        `env:"SIDECAR_ANIME365_LOGIN,required,notEmpty"`
	Anime365Password              string        `env:"SIDECAR_ANIME365_PASSWORD,required,notEmpty"`
	EmbyAPIKey                    string        `env:"SIDECAR_EMBY_API_KEY,required,notEmpty"`
	EmbyLibraryName               string        `env:"SIDECAR_EMBY_LIBRARY_NAME,required,notEmpty"`
	EmbyUserID                    string        `env:"SIDECAR_EMBY_USER_ID,required,notEmpty"`
	LibraryDirectory              string        `env:"SIDECAR_LIBRARY_DIRECTORY,required,notEmpty"`
	ScanSources                   []string      `env:"SIDECAR_SCAN_SOURCES,required,notEmpty"                     envDefault:"list_watching"`
	Translations                  []string      `env:"SIDECAR_TRANSLATIONS,required,notEmpty"                     envDefault:"ru_subtitles,ru_dub"`
	ScanIdleInterval              time.Duration `env:"SIDECAR_SCAN_IDLE_INTERVAL,required,notEmpty"               envDefault:"5m"`
	MetadataRefreshIdleInterval   time.Duration `env:"SIDECAR_METADATA_REFRESH_IDLE_INTERVAL,required,notEmpty"   envDefault:"1h"`
	NotifyEpisodesWatchedInterval time.Duration `env:"SIDECAR_NOTIFY_EPISODES_WATCHED_INTERVAL,required,notEmpty" envDefault:"5m"`
}
