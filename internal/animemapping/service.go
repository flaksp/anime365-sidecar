package animemapping

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	requestTimeout          = time.Minute
	maximumDatabaseFileSize = 100 << 20
)

type ExternalIDs struct {
	IMDb             string
	TheMovieDatabase string
	TheTVDB          string
}

type databaseEntry struct {
	IMDbID       []string `json:"imdb_id"`
	TheMovieDBID struct {
		Movie []int64 `json:"movie"`
		TV    int64   `json:"tv"`
	} `json:"themoviedb_id"`
	MyAnimeListID int64 `json:"mal_id"`
	TheTVDB       int64 `json:"tvdb_id"`
}

type Service struct {
	databaseURL  *url.URL
	httpClient   *http.Client
	logger       *slog.Logger
	index        map[int64]ExternalIDs
	refreshMutex sync.Mutex
	indexMutex   sync.RWMutex
}

func NewService(
	databaseURL *url.URL,
	httpClient *http.Client,
	logger *slog.Logger,
) (*Service, error) {
	if databaseURL == nil {
		return nil, errors.New("anime mapping database URL is required")
	}

	return &Service{
		databaseURL: databaseURL,
		httpClient:  httpClient,
		logger:      logger,
		index:       make(map[int64]ExternalIDs),
	}, nil
}

func (s *Service) LookupByMyAnimeListID(myAnimeListID int64) (ExternalIDs, bool) {
	s.indexMutex.RLock()
	defer s.indexMutex.RUnlock()

	externalIDs, exists := s.index[myAnimeListID]

	return externalIDs, exists
}

func (s *Service) Refresh(ctx context.Context) error {
	s.refreshMutex.Lock()
	defer s.refreshMutex.Unlock()

	requestContext, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	httpRequest, err := http.NewRequestWithContext(requestContext, http.MethodGet, s.databaseURL.String(), nil)
	if err != nil {
		return fmt.Errorf("creating anime mapping database request: %w", err)
	}

	httpResponse, err := s.httpClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("downloading anime mapping database: %w", err)
	}
	defer func() {
		if err := httpResponse.Body.Close(); err != nil && s.logger != nil {
			s.logger.WarnContext(
				requestContext,
				"Anime mapping database response body closed unexpectedly",
				slog.String("error", err.Error()),
			)
		}
	}()

	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("downloading anime mapping database: unexpected HTTP status %s", httpResponse.Status)
	}

	index, err := decodeDatabase(io.LimitReader(httpResponse.Body, maximumDatabaseFileSize))
	if err != nil {
		return fmt.Errorf("decoding anime mapping database: %w", err)
	}

	if len(index) == 0 {
		return errors.New("decoding anime mapping database: database contains no MyAnimeList mappings")
	}

	s.indexMutex.Lock()
	s.index = index
	s.indexMutex.Unlock()

	if s.logger != nil {
		s.logger.InfoContext(
			ctx,
			"Anime mapping database loaded",
			slog.Int("my_anime_list_id_count", len(index)),
			slog.String("url", s.databaseURL.String()),
		)
	}

	return nil
}

func decodeDatabase(reader io.Reader) (map[int64]ExternalIDs, error) {
	decoder := json.NewDecoder(reader)

	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	if delimiter, ok := token.(json.Delim); !ok || delimiter != '[' {
		return nil, errors.New("expected a JSON array")
	}

	index := make(map[int64]ExternalIDs)
	theMovieDBIDIsForTV := make(map[int64]bool)

	for decoder.More() {
		var entry databaseEntry
		if err := decoder.Decode(&entry); err != nil {
			return nil, err
		}

		if entry.MyAnimeListID <= 0 {
			continue
		}

		externalIDs := index[entry.MyAnimeListID]

		if externalIDs.IMDb == "" {
			for _, imdbID := range entry.IMDbID {
				if imdbID != "" {
					externalIDs.IMDb = imdbID

					break
				}
			}
		}

		if entry.TheMovieDBID.TV > 0 {
			if !theMovieDBIDIsForTV[entry.MyAnimeListID] {
				externalIDs.TheMovieDatabase = strconv.FormatInt(entry.TheMovieDBID.TV, 10)
				theMovieDBIDIsForTV[entry.MyAnimeListID] = true
			}
		} else if externalIDs.TheMovieDatabase == "" {
			for _, movieID := range entry.TheMovieDBID.Movie {
				if movieID > 0 {
					externalIDs.TheMovieDatabase = strconv.FormatInt(movieID, 10)

					break
				}
			}
		}

		if externalIDs.TheTVDB == "" && entry.TheTVDB > 0 {
			externalIDs.TheTVDB = strconv.FormatInt(entry.TheTVDB, 10)
		}

		index[entry.MyAnimeListID] = externalIDs
	}

	if _, err := decoder.Token(); err != nil {
		return nil, err
	}

	return index, nil
}
