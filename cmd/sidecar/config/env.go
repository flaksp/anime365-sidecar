package config

import (
	"log/slog"
	"net/url"
	"time"
)

type Env struct {
	Anime365BaseURL             *url.URL      `env:"SIDECAR_ANIME365_BASE_URL,required,notEmpty"`
	EmbyBaseURL                 *url.URL      `env:"SIDECAR_EMBY_BASE_URL,required,notEmpty"`
	ShikimoriBaseURL            *url.URL      `env:"SIDECAR_SHIKIMORI_BASE_URL,required,notEmpty"             envDefault:"https://shikimori.io"`
	JikanAPIBaseURL             *url.URL      `env:"SIDECAR_JIKAN_API_BASE_URL,required,notEmpty"             envDefault:"https://api.jikan.moe"`
	TelegramBotAPICredentials   *url.URL      `env:"SIDECAR_TELEGRAM_BOT_API_CREDENTIALS"`
	EmbyUserID                  string        `env:"SIDECAR_EMBY_USER_ID,required,notEmpty"`
	EmbyAPIKey                  string        `env:"SIDECAR_EMBY_API_KEY,required,notEmpty"`
	EmbyLibraryName             string        `env:"SIDECAR_EMBY_LIBRARY_NAME,required,notEmpty"`
	Anime365Password            string        `env:"SIDECAR_ANIME365_PASSWORD,required,notEmpty"`
	LibraryDirectory            string        `env:"SIDECAR_LIBRARY_DIRECTORY,required,notEmpty"`
	Anime365Login               string        `env:"SIDECAR_ANIME365_LOGIN,required,notEmpty"`
	ScanSources                 []string      `env:"SIDECAR_SCAN_SOURCES,required,notEmpty"                   envDefault:"list_watching"`
	Translations                []string      `env:"SIDECAR_TRANSLATIONS,required,notEmpty"                   envDefault:"ru_subtitles,ru_dub"`
	DownloadTimeoutVideo        time.Duration `env:"SIDECAR_DOWNLOAD_TIMEOUT_VIDEO,required,notEmpty"         envDefault:"1h"`
	ScanIdleInterval            time.Duration `env:"SIDECAR_SCAN_IDLE_INTERVAL,required,notEmpty"             envDefault:"5m"`
	MetadataRefreshIdleInterval time.Duration `env:"SIDECAR_METADATA_REFRESH_IDLE_INTERVAL,required,notEmpty" envDefault:"1h"`
	LogLevel                    slog.Level    `env:"SIDECAR_LOG_LEVEL,required,notEmpty"                      envDefault:"INFO"`
	DownloadTimeoutImage        time.Duration `env:"SIDECAR_DOWNLOAD_TIMEOUT_IMAGE,required,notEmpty"         envDefault:"1m"`
}
