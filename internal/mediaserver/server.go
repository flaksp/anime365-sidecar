// Package mediaserver defines the application-facing media server facade.
// Implementations own provider API calls, metadata mapping, library layout,
// playback checks, and public item URLs. No provider SDK types cross this boundary.
package mediaserver

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/show"
)

var ErrItemNotFound = errors.New("media server item not found")

// Server exposes the library operations used by the sidecar in Anime365 terms.
// Item identifiers and API models specific to a provider stay in its adapter.
// Persistence is an implementation detail; adapters may use a manifest, a
// database, or no local storage. Initialize must finish before workers start
// using the library.
type Server interface {
	Initialize(ctx context.Context) error
	CreateShowIfNotExists(showID show.Anime365SeriesID, showTitle string, myAnimeListID show.MyAnimeListID) error
	GetIDs() map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{}
	GetTranslationQuality(
		showID show.Anime365SeriesID,
		episodeID episode.Anime365EpisodeID,
		translationID episode.Anime365TranslationID,
	) (int64, bool)
	ComputeTranslationFileAbsolutePathsForDownloads(
		showEntity show.Show,
		episodeEntity episode.Episode,
		translationEntity episode.Translation,
		translationMedia episode.TranslationMedia,
	) (string, string, error)
	SaveTranslationPaths(
		ctx context.Context,
		showID show.Anime365SeriesID,
		episodeID episode.Anime365EpisodeID,
		translationID episode.Anime365TranslationID,
		videoFileAbsolutePath string,
		subtitlesFileAbsolutePath string,
		height int64,
	) error
	RefreshLibrary(ctx context.Context) error

	// Deletion methods return false without deleting files when playback or the
	// retention period prevents removal. Provider lookup failures must fail closed.
	DeleteTranslationIfNotPlaying(
		ctx context.Context,
		showID show.Anime365SeriesID,
		episodeID episode.Anime365EpisodeID,
		translationID episode.Anime365TranslationID,
	) (bool, error)
	DeleteEpisodeIfStale(
		ctx context.Context,
		showID show.Anime365SeriesID,
		episodeID episode.Anime365EpisodeID,
		deleteAfter time.Duration,
	) (bool, error)

	GetIDsWithoutMetadata(
		ctx context.Context,
	) (map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{}, error)
	GetIDsWithMetadata(
		ctx context.Context,
	) (map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{}, error)
	InitializeShowMetadata(ctx context.Context, showEntity show.Show) error
	UpdateShowMetadata(ctx context.Context, showEntity show.Show) error
	UpdateSeasonMetadata(ctx context.Context, showEntity show.Show) error
	UpdateShowMetadataFromShikimori(
		ctx context.Context,
		showID show.Anime365SeriesID,
		metadata show.ShowFromShikimori,
	) error
	InitializeTranslationMetadata(
		ctx context.Context,
		showID show.Anime365SeriesID,
		episodeEntity episode.Episode,
		translationEntity episode.Translation,
	) error
	UpdateTranslationMetadata(
		ctx context.Context,
		showID show.Anime365SeriesID,
		episodeEntity episode.Episode,
		translationEntity episode.Translation,
	) error

	IsPosterExists(showID show.Anime365SeriesID, posterURL *url.URL) (bool, error)
	ComputePosterFileAbsolutePath(showID show.Anime365SeriesID, posterURL *url.URL) (string, error)
	IsBackdropExists(showID show.Anime365SeriesID, screenshotID string) bool
	ComputeBackdropFileAbsolutePath(showID show.Anime365SeriesID, imageURL *url.URL) (string, error)
	AddBackdrop(showID show.Anime365SeriesID, screenshotID string) error

	GetLastWatchedEpisodeNumber(
		ctx context.Context,
		showID show.Anime365SeriesID,
	) (int64, episode.Anime365TranslationID, error)
	GetEpisodeMetadataForNotification(
		ctx context.Context,
		showID show.Anime365SeriesID,
		episodeID episode.Anime365EpisodeID,
		translationID episode.Anime365TranslationID,
	) (EpisodeMetadataForNotification, error)
}

type EpisodeMetadataForNotification struct {
	WebURL                    *url.URL
	VideoMetadataDisplayTitle string
	SeriesName                string
	EpisodeLabel              string
	Bitrate                   int64
}
