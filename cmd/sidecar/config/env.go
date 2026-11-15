package config

import (
	"log/slog"
	"net/url"
	"time"

	"github.com/flaksp/anime365-sidecar/internal/librarycleanup"
)

type Env struct {
	MediaServer                           string                `env:"SIDECAR_MEDIA_SERVER,notEmpty"                                        envDefault:"emby"`
	JellyfinBaseURL                       *url.URL              `env:"SIDECAR_JELLYFIN_BASE_URL"`
	JellyfinPublicBaseURL                 *url.URL              `env:"SIDECAR_JELLYFIN_PUBLIC_BASE_URL"`
	JellyfinAPIKey                        string                `env:"SIDECAR_JELLYFIN_API_KEY"`
	JellyfinLibraryID                     string                `env:"SIDECAR_JELLYFIN_LIBRARY_ID"`
	JellyfinUserID                        string                `env:"SIDECAR_JELLYFIN_USER_ID"`
	Anime365BaseURL                       *url.URL              `env:"SIDECAR_ANIME365_BASE_URL,required,notEmpty"`
	EmbyBaseURL                           *url.URL              `env:"SIDECAR_EMBY_BASE_URL"`
	ShikimoriBaseURL                      *url.URL              `env:"SIDECAR_SHIKIMORI_BASE_URL,required,notEmpty"                         envDefault:"https://shikimori.io"`
	AnimeListDatabaseURL                  *url.URL              `env:"SIDECAR_ANIME_LIST_DATABASE_URL,required,notEmpty"                    envDefault:"https://raw.githubusercontent.com/Fribb/anime-lists/master/anime-list-full.json"`
	TelegramBotAPICredentials             *url.URL              `env:"SIDECAR_TELEGRAM_BOT_API_CREDENTIALS"`
	EmbyPublicBaseURL                     *url.URL              `env:"SIDECAR_EMBY_PUBLIC_BASE_URL"`
	EmbyAPIKey                            string                `env:"SIDECAR_EMBY_API_KEY"`
	EmbyLibraryID                         string                `env:"SIDECAR_EMBY_LIBRARY_ID"`
	EmbyUserID                            string                `env:"SIDECAR_EMBY_USER_ID"`
	Anime365Password                      string                `env:"SIDECAR_ANIME365_PASSWORD,required,notEmpty"`
	Anime365Login                         string                `env:"SIDECAR_ANIME365_LOGIN,required,notEmpty"`
	LibraryDirectory                      string                `env:"SIDECAR_LIBRARY_DIRECTORY,required,notEmpty"`
	TemporaryDirectory                    string                `env:"SIDECAR_TEMPORARY_DIRECTORY"`
	WatchedEpisodesCleanupPolicy          librarycleanup.Policy `env:"SIDECAR_WATCHED_EPISODES_CLEANUP_POLICY,notEmpty"                     envDefault:"do_not_delete"`
	Translations                          []string              `env:"SIDECAR_TRANSLATIONS,required,notEmpty"                               envDefault:"ru_subtitles,ru_dub"`
	ScanSources                           []string              `env:"SIDECAR_SCAN_SOURCES,required,notEmpty"                               envDefault:"list_watching"`
	BlacklistedTranslationAuthors         []string              `env:"SIDECAR_BLACKLISTED_TRANSLATION_AUTHORS"`
	PreferredTranslationAuthors           []string              `env:"SIDECAR_PREFERRED_TRANSLATION_AUTHORS"`
	WatchedEpisodesCleanupAfter           time.Duration         `env:"SIDECAR_WATCHED_EPISODES_CLEANUP_AFTER,notEmpty"                      envDefault:"336h"`
	DownloadTimeoutVideo                  time.Duration         `env:"SIDECAR_DOWNLOAD_TIMEOUT_VIDEO,required,notEmpty"                     envDefault:"1h"`
	ScanIdleInterval                      time.Duration         `env:"SIDECAR_SCAN_IDLE_INTERVAL,required,notEmpty"                         envDefault:"5m"`
	MetadataRefreshIdleInterval           time.Duration         `env:"SIDECAR_METADATA_REFRESH_IDLE_INTERVAL,required,notEmpty"             envDefault:"1h"`
	FreshItemsMetadataRefreshIdleInterval time.Duration         `env:"SIDECAR_FRESH_ITEMS_METADATA_REFRESH_IDLE_INTERVAL,required,notEmpty" envDefault:"1m"`
	AnimeListRefreshIdleInterval          time.Duration         `env:"SIDECAR_ANIME_LIST_REFRESH_IDLE_INTERVAL,required,notEmpty"           envDefault:"24h"`
	LogLevel                              slog.Level            `env:"SIDECAR_LOG_LEVEL,required,notEmpty"                                  envDefault:"INFO"`
	DownloadTimeoutImage                  time.Duration         `env:"SIDECAR_DOWNLOAD_TIMEOUT_IMAGE,required,notEmpty"                     envDefault:"1m"`
	EpisodesToDownloadAhead               uint32                `env:"SIDECAR_EPISODES_TO_DOWNLOAD_AHEAD,required,notEmpty"                 envDefault:"50"`
	DeleteRemovedTranslations             bool                  `env:"SIDECAR_DELETE_REMOVED_TRANSLATIONS"                                  envDefault:"false"`
}
