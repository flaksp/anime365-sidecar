package libraryfiles

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/authorslistformatter"
	"github.com/flaksp/anime365-sidecar/pkg/filename"
	"github.com/flaksp/anime365-sidecar/pkg/filesystemutils"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// Library is an optional file-backed storage helper for media server adapters.
// It is not part of the application-facing mediaserver contract.
type Library struct {
	store     Store
	Directory string
}

func NewLibrary(directory string, store Store) *Library {
	return &Library{store: store, Directory: directory}
}

var errNotDirectory = errors.New("not a directory")

func (s *Library) CreateShowIfNotExists(
	showID show.Anime365SeriesID,
	showTitle string,
	myAnimeListID show.MyAnimeListID,
) error {
	showDirectoryName, showManifestEntryExists := s.store.GetShowDirectoryName(showID)
	if !showManifestEntryExists {
		computedShowDirectoryName := filename.Clean(showTitle)
		computedShowDirectoryName = strings.Join(strings.Fields(computedShowDirectoryName), " ")
		computedShowDirectoryName = strings.TrimSpace(computedShowDirectoryName)
		computedShowDirectoryName = fmt.Sprintf("%s [mal-%d]", computedShowDirectoryName, myAnimeListID)

		showDirectoryName = computedShowDirectoryName
	}

	showDirectoryAbsolutePath := filepath.Join(s.Directory, showDirectoryName)

	if err := s.CreateDirectoryIfNotExists(showDirectoryAbsolutePath); err != nil {
		return fmt.Errorf("create show directory: %w", err)
	}

	if err := s.CreateDirectoryIfNotExists(filepath.Join(showDirectoryAbsolutePath, "Season 01")); err != nil {
		return fmt.Errorf("create season directory: %w", err)
	}

	if !showManifestEntryExists {
		if err := s.store.InsertShowEntry(showID, showDirectoryName, myAnimeListID); err != nil {
			return fmt.Errorf("failed to set show entry: %w", err)
		}
	}

	return nil
}

func (s *Library) GetTranslationQuality(
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
) (int64, bool) {
	return s.store.GetTranslationQuality(
		showID,
		episodeID,
		translationID,
	)
}

func (s *Library) GetTranslationIDs(
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
) map[episode.Anime365TranslationID]struct{} {
	return s.store.GetTranslationIDs(showID, episodeID)
}

func (s *Library) DeleteTranslation(
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
) error {
	videoFileRelativePath, subtitlesFileRelativePath, exists := s.store.GetTranslationRelativePaths(
		showID,
		episodeID,
		translationID,
	)
	if exists {
		videoFileAbsolutePath := filepath.Join(s.Directory, videoFileRelativePath)

		if err := filesystemutils.DeleteFileIfExists(videoFileAbsolutePath); err != nil {
			return fmt.Errorf("delete video file: %w", err)
		}

		if subtitlesFileRelativePath != "" {
			subtitlesFileAbsolutePath := filepath.Join(s.Directory, subtitlesFileRelativePath)

			if err := filesystemutils.DeleteFileIfExists(subtitlesFileAbsolutePath); err != nil {
				return fmt.Errorf("delete subtitle file: %w", err)
			}
		}
	}

	if err := s.store.DeleteTranslation(showID, episodeID, translationID); err != nil {
		return fmt.Errorf("delete translation from manifest: %w", err)
	}

	return nil
}

func (s *Library) ComputeTranslationFileAbsolutePathsForDownloads(
	showEntity show.Show,
	episodeEntity episode.Episode,
	translationEntity episode.Translation,
	translationMedia episode.TranslationMedia,
) (string, string, error) {
	showDirectoryName, exists := s.store.GetShowDirectoryName(showEntity.Anime365ID)
	if !exists {
		return "", "", errors.New("show does not exists in manifest")
	}

	var (
		translationFileName              string
		translationDirectoryRelativePath string
	)

	if episodeEntity.IsTrailer || episodeEntity.IsSpecial {
		if episodeEntity.IsTrailer {
			translationDirectoryRelativePath = filepath.Join(showDirectoryName, "Season 01", "trailers")
		} else {
			translationDirectoryRelativePath = filepath.Join(showDirectoryName, "Season 00")
		}

		translationFileName = fmt.Sprintf(
			"%s - %s %s by %s, t%d",
			episodeEntity.EpisodeLabel,
			display.English.Languages().Name(translationEntity.Variant.Language),
			cases.Title(language.English).String(translationEntity.Variant.Kind.Label()),
			formatAuthorsListForFileName(translationEntity.Authors),
			translationEntity.Anime365ID,
		)
	} else {
		translationFileName = fmt.Sprintf(
			"%s S1E%d - %s %s by %s, t%d",
			showEntity.TitleRomaji,
			episodeEntity.EpisodeNumber,
			display.English.Languages().Name(translationEntity.Variant.Language),
			cases.Title(language.English).String(translationEntity.Variant.Kind.Label()),
			formatAuthorsListForFileName(translationEntity.Authors),
			translationEntity.Anime365ID,
		)

		translationDirectoryRelativePath = filepath.Join(showDirectoryName, "Season 01")
	}

	translationDirectoryAbsolutePath := filepath.Join(s.Directory, translationDirectoryRelativePath)
	if err := s.CreateDirectoryIfNotExists(translationDirectoryAbsolutePath); err != nil {
		return "", "", fmt.Errorf("failed to create translation directory: %w", err)
	}

	translationFileName = filename.Clean(translationFileName)
	translationFileName = strings.Join(strings.Fields(translationFileName), " ")
	translationFileName = strings.TrimSpace(translationFileName)

	videoFileRelativePath := filepath.Join(translationDirectoryRelativePath, translationFileName+".mp4")
	videoFileAbsolutePath := filepath.Join(s.Directory, videoFileRelativePath)

	var (
		subtitlesFileRelativePath string
		subtitlesFileAbsolutePath string
	)

	if translationMedia.SubtitlesURL != nil {
		translationLanguage, _ := translationEntity.Variant.Language.Base()

		subtitlesFileRelativePath = filepath.Join(
			translationDirectoryRelativePath,
			fmt.Sprintf(
				"%s.%s.ass",
				translationFileName,
				translationLanguage.String(),
			),
		)

		subtitlesFileAbsolutePath = filepath.Join(s.Directory, subtitlesFileRelativePath)
	}

	return videoFileAbsolutePath, subtitlesFileAbsolutePath, nil
}

func (s *Library) GetIDs() map[show.Anime365SeriesID]map[episode.Anime365EpisodeID]map[episode.Anime365TranslationID]struct{} {
	return s.store.GetIDs()
}

func (s *Library) GetMyAnimeListIDToShowIDMap() map[show.MyAnimeListID]show.Anime365SeriesID {
	return s.store.GetMyAnimeListIDToShowIDMap()
}

func (s *Library) IsPosterExists(showID show.Anime365SeriesID, posterURL *url.URL) (bool, error) {
	fileAbsolutePath, err := s.ComputePosterFileAbsolutePath(showID, posterURL)
	if err != nil {
		return false, fmt.Errorf("failed to compute poster file absolute path: %w", err)
	}

	fileExists, err := filesystemutils.FileExists(fileAbsolutePath)
	if err != nil {
		return false, fmt.Errorf("failed to check if file exists: %w", err)
	}

	return fileExists, nil
}

func (s *Library) ComputePosterFileAbsolutePath(showID show.Anime365SeriesID, posterURL *url.URL) (string, error) {
	showDirectoryName, exists := s.store.GetShowDirectoryName(showID)
	if !exists {
		return "", errors.New("show directory not found")
	}

	fileAbsolutePath := filepath.Join(s.Directory, showDirectoryName, "poster"+filepath.Ext(posterURL.Path))

	return fileAbsolutePath, nil
}

func (s *Library) IsBackdropExists(
	showID show.Anime365SeriesID,
	screenshotID string,
) bool {
	return s.store.IsBackdropExists(showID, screenshotID)
}

func (s *Library) ComputeBackdropFileAbsolutePath(
	showID show.Anime365SeriesID,
	imageURL *url.URL,
) (string, error) {
	showDirectoryName, exists := s.store.GetShowDirectoryName(showID)
	if !exists {
		return "", errors.New("show directory not found")
	}

	backdropCount := s.store.BackdropCount(showID)

	fileAbsolutePath := filepath.Join(
		s.Directory,
		showDirectoryName,
		fmt.Sprintf("backdrop%d%s", backdropCount+1, filepath.Ext(imageURL.Path)),
	)

	return fileAbsolutePath, nil
}

func (s *Library) AddBackdrop(
	showID show.Anime365SeriesID,
	screenshotID string,
) error {
	err := s.store.AddBackdrop(showID, screenshotID)
	if err != nil {
		return errors.New("failed to add backdrop to manifest")
	}

	return nil
}

func (s *Library) GetShowIDByDirectoryName(directoryName string) (show.Anime365SeriesID, bool) {
	return s.store.GetShowIDByDirectoryName(directoryName)
}

func (s *Library) GetIDsByVideoFileRelativePath(
	videoFileRelativePath string,
) (show.Anime365SeriesID, episode.Anime365EpisodeID, episode.Anime365TranslationID, bool) {
	return s.store.GetIDsByVideoFileRelativePath(videoFileRelativePath)
}

func (s *Library) directoryExists(absolutePath string) (bool, error) {
	info, err := os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to check directory: %w", err)
	}

	if info.IsDir() {
		return true, nil
	}

	return false, errNotDirectory
}

func (s *Library) CreateDirectoryIfNotExists(absolutePath string) error {
	directoryExists, err := s.directoryExists(absolutePath)
	if err != nil {
		return fmt.Errorf("failed to check directory exists: %w", err)
	}

	if directoryExists {
		return nil
	}

	if err := os.Mkdir(absolutePath, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

func formatAuthorsListForFileName(authorsList []string) string {
	formattedAuthorsList := authorslistformatter.Format(authorsList)

	replacer := strings.NewReplacer("(", " ", ")", " ", "[", " ", "]", " ", " - ", " ")

	formattedAuthorsList = replacer.Replace(formattedAuthorsList)
	formattedAuthorsList = strings.Join(strings.Fields(formattedAuthorsList), " ")
	formattedAuthorsList = strings.TrimSpace(formattedAuthorsList)

	return formattedAuthorsList
}
