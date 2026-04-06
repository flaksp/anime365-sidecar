package manifest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strconv"
	"sync"

	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/show"
)

type showManifestEntry struct {
	Screenshots   map[string]bool                 `json:"screenshots"`
	Episodes      map[string]episodeManifestEntry `json:"episodes"`
	DirectoryName string                          `json:"directory_name"`
	MyAnimeListID int64                           `json:"my_anime_list_id,omitempty"`
}

type episodeManifestEntry struct {
	Translations map[string]translationManifestEntry `json:"translations"`
}

type translationManifestEntry struct {
	VideoFileRelativePath     string `json:"video_file_relative_path"`
	SubtitlesFileRelativePath string `json:"subtitles_file_relative_path,omitempty"`
	Height                    int64  `json:"height"`
}

type manifest struct {
	Shows map[string]showManifestEntry `json:"shows"`
}

func NewService(downloadsDirectory string, logger *slog.Logger) *Service {
	return &Service{
		downloadsDirectory: downloadsDirectory,
		inMemoryManifest: &manifest{
			Shows: make(map[string]showManifestEntry),
		},
		logger: logger,
	}
}

type Service struct {
	inMemoryManifest   *manifest
	logger             *slog.Logger
	downloadsDirectory string
	mu                 sync.RWMutex
	manifestFileMu     sync.RWMutex
}

func (s *Service) LoadFromDisk(ctx context.Context) error {
	s.manifestFileMu.RLock()
	defer s.manifestFileMu.RUnlock()

	data, err := os.ReadFile(s.manifestFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			s.logger.InfoContext(
				ctx,
				"Manifest file not found; starting with clean database",
				slog.String("path", s.downloadsDirectory),
			)

			return nil
		}

		s.logger.ErrorContext(ctx,
			"Failed to read manifest file",
			slog.String("path", s.downloadsDirectory),
			slog.Any("error", err),
		)

		return err
	}

	err = json.Unmarshal(data, s.inMemoryManifest)
	if err != nil {
		s.logger.ErrorContext(ctx,
			"Failed to parse JSON in manifest file",
			slog.Any("error", err),
			slog.String("path", s.downloadsDirectory),
			slog.String("content", string(data)),
		)

		return err
	}

	s.logger.InfoContext(ctx, "Manifest file loaded successfully")

	return nil
}

func (s *Service) GetShowDirectoryName(
	showID show.Anime365SeriesID,
) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	showIDStr := strconv.FormatInt(int64(showID), 10)

	showEntry, exists := s.inMemoryManifest.Shows[showIDStr]
	if !exists {
		return "", false
	}

	return showEntry.DirectoryName, true
}

func (s *Service) SetShowEntry(
	showID show.Anime365SeriesID,
	directoryName string,
	myAnimeListID show.MyAnimeListID,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	showIDStr := strconv.FormatInt(int64(showID), 10)

	s.inMemoryManifest.Shows[showIDStr] = showManifestEntry{
		Screenshots:   make(map[string]bool),
		Episodes:      make(map[string]episodeManifestEntry),
		DirectoryName: directoryName,
		MyAnimeListID: int64(myAnimeListID),
	}

	if err := s.saveToDisk(); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	return nil
}

func (s *Service) GetTranslationRelativePaths(
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
) (string, string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	showIDStr := strconv.FormatInt(int64(showID), 10)
	episodeIDStr := strconv.FormatInt(int64(episodeID), 10)
	translationIDStr := strconv.FormatInt(int64(translationID), 10)

	showEntry, exists := s.inMemoryManifest.Shows[showIDStr]
	if !exists {
		return "", "", false
	}

	episodeEntry, exists := showEntry.Episodes[episodeIDStr]
	if !exists {
		return "", "", false
	}

	translationEntry, exists := episodeEntry.Translations[translationIDStr]
	if !exists {
		return "", "", false
	}

	return translationEntry.VideoFileRelativePath, translationEntry.SubtitlesFileRelativePath, true
}

func (s *Service) GetTranslationQuality(
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	showIDStr := strconv.FormatInt(int64(showID), 10)
	episodeIDStr := strconv.FormatInt(int64(episodeID), 10)
	translationIDStr := strconv.FormatInt(int64(translationID), 10)

	showEntry, exists := s.inMemoryManifest.Shows[showIDStr]
	if !exists {
		return 0, false
	}

	episodeEntry, exists := showEntry.Episodes[episodeIDStr]
	if !exists {
		return 0, false
	}

	translationEntry, exists := episodeEntry.Translations[translationIDStr]
	if !exists {
		return 0, false
	}

	return translationEntry.Height, true
}

func (s *Service) SetTranslationEntry(
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
	videoFileRelativePath string,
	subtitlesFileRelativePath string,
	height int64,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	showIDStr := strconv.FormatInt(int64(showID), 10)
	episodeIDStr := strconv.FormatInt(int64(episodeID), 10)
	translationIDStr := strconv.FormatInt(int64(translationID), 10)

	showEntry, exists := s.inMemoryManifest.Shows[showIDStr]
	if !exists {
		return fmt.Errorf("no show found with show ID %d", showID)
	}

	episodeEntry, exists := showEntry.Episodes[episodeIDStr]
	if !exists {
		episodeEntry = episodeManifestEntry{
			Translations: make(map[string]translationManifestEntry),
		}
	}

	translationEntry := translationManifestEntry{
		VideoFileRelativePath: videoFileRelativePath,
		Height:                height,
	}

	if subtitlesFileRelativePath != "" {
		translationEntry.SubtitlesFileRelativePath = subtitlesFileRelativePath
	}

	// Add or update translation
	episodeEntry.Translations[translationIDStr] = translationEntry

	// Write back episode entry
	showEntry.Episodes[episodeIDStr] = episodeEntry

	// Write back show entry
	s.inMemoryManifest.Shows[showIDStr] = showEntry

	if err := s.saveToDisk(); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	return nil
}

func (s *Service) DeleteTranslation(
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	showIDStr := strconv.FormatInt(int64(showID), 10)
	episodeIDStr := strconv.FormatInt(int64(episodeID), 10)
	translationIDStr := strconv.FormatInt(int64(translationID), 10)

	showEntry, exists := s.inMemoryManifest.Shows[showIDStr]
	if !exists {
		return nil
	}

	episodeEntry, exists := showEntry.Episodes[episodeIDStr]
	if !exists {
		return nil
	}

	delete(episodeEntry.Translations, translationIDStr)

	// Write back episode entry
	showEntry.Episodes[episodeIDStr] = episodeEntry

	// Write back show entry
	s.inMemoryManifest.Shows[showIDStr] = showEntry

	if err := s.saveToDisk(); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	return nil
}

func (s *Service) GetIDs() map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(
		map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{},
		len(s.inMemoryManifest.Shows),
	)

	for showIDStr, showEntry := range s.inMemoryManifest.Shows {
		showIDInt, err := strconv.Atoi(showIDStr)
		if err != nil {
			continue
		}

		showID := show.Anime365SeriesID(showIDInt)

		if _, ok := result[showID]; !ok {
			result[showID] = make(
				map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{},
				len(showEntry.Episodes),
			)
		}

		for episodeIDStr, episodeEntry := range showEntry.Episodes {
			episodeIDInt, err := strconv.Atoi(episodeIDStr)
			if err != nil {
				continue
			}

			episodeID := episode.Anime365EpisodeID(episodeIDInt)

			if _, ok := result[showID][episodeID]; !ok {
				result[showID][episodeID] = make(
					map[episode.Anime365TranslationID]struct{},
					len(episodeEntry.Translations),
				)
			}

			for translationIDStr := range episodeEntry.Translations {
				translationIDInt, err := strconv.Atoi(translationIDStr)
				if err != nil {
					continue
				}

				translationID := episode.Anime365TranslationID(translationIDInt)

				result[showID][episodeID][translationID] = struct{}{}
			}
		}
	}

	return result
}

func (s *Service) GetMyAnimeListIDToShowIDMap() map[show.MyAnimeListID]show.Anime365SeriesID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[show.MyAnimeListID]show.Anime365SeriesID, len(s.inMemoryManifest.Shows))

	for showIDStr, showEntry := range s.inMemoryManifest.Shows {
		if showEntry.MyAnimeListID == 0 {
			continue
		}

		showIDInt, err := strconv.Atoi(showIDStr)
		if err != nil {
			continue
		}

		result[show.MyAnimeListID(showEntry.MyAnimeListID)] = show.Anime365SeriesID(showIDInt)
	}

	return result
}

func (s *Service) IsBackdropExists(
	showID show.Anime365SeriesID,
	screenshotID string,
) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	showIDStr := strconv.FormatInt(int64(showID), 10)

	showEntry, exists := s.inMemoryManifest.Shows[showIDStr]
	if !exists {
		return false
	}

	_, exists = showEntry.Screenshots[screenshotID]

	return exists
}

func (s *Service) BackdropCount(
	showID show.Anime365SeriesID,
) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	showIDStr := strconv.FormatInt(int64(showID), 10)

	showEntry, exists := s.inMemoryManifest.Shows[showIDStr]
	if !exists {
		return 0
	}

	return len(showEntry.Screenshots)
}

func (s *Service) AddBackdrop(
	showID show.Anime365SeriesID,
	screenshotID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	showIDStr := strconv.FormatInt(int64(showID), 10)

	showEntry, exists := s.inMemoryManifest.Shows[showIDStr]
	if !exists {
		return errors.New("show entry does not exist")
	}

	showEntry.Screenshots[screenshotID] = true

	// Write back show entry
	s.inMemoryManifest.Shows[showIDStr] = showEntry

	// Persist manifest
	if err := s.saveToDisk(); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	return nil
}

func (s *Service) saveToDisk() error {
	s.manifestFileMu.Lock()
	defer s.manifestFileMu.Unlock()

	data, err := json.MarshalIndent(s.inMemoryManifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	return os.WriteFile(s.manifestFilePath(), data, 0o644)
}

func (s *Service) manifestFilePath() string {
	return path.Join(s.downloadsDirectory, "manifest.json")
}
