package libraryfiles

import (
	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/show"
)

// Store supplies file-management state to Library. Serialization, provider IDs,
// ownership validation, and migration policy belong to each adapter.
type Store interface {
	GetShowDirectoryName(
		showID show.Anime365SeriesID,
	) (string, bool)
	InsertShowEntry(
		showID show.Anime365SeriesID,
		directoryName string,
		myAnimeListID show.MyAnimeListID,
	) error
	GetTranslationRelativePaths(
		showID show.Anime365SeriesID,
		episodeID episode.Anime365EpisodeID,
		translationID episode.Anime365TranslationID,
	) (string, string, bool)
	GetTranslationQuality(
		showID show.Anime365SeriesID,
		episodeID episode.Anime365EpisodeID,
		translationID episode.Anime365TranslationID,
	) (int64, bool)
	GetTranslationIDs(
		showID show.Anime365SeriesID,
		episodeID episode.Anime365EpisodeID,
	) map[episode.Anime365TranslationID]struct{}
	DeleteTranslation(
		showID show.Anime365SeriesID,
		episodeID episode.Anime365EpisodeID,
		translationID episode.Anime365TranslationID,
	) error
	GetIDs() map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{}
	GetMyAnimeListIDToShowIDMap() map[show.MyAnimeListID]show.Anime365SeriesID
	IsBackdropExists(
		showID show.Anime365SeriesID,
		screenshotID string,
	) bool
	BackdropCount(
		showID show.Anime365SeriesID,
	) int
	AddBackdrop(
		showID show.Anime365SeriesID,
		screenshotID string,
	) error
	GetShowIDByDirectoryName(directoryName string) (show.Anime365SeriesID, bool)
	GetIDsByVideoFileRelativePath(
		videoFileRelativePath string,
	) (show.Anime365SeriesID, episode.Anime365EpisodeID, episode.Anime365TranslationID, bool)
}
