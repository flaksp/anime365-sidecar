package show

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/pkg/anime365client"
)

type (
	Anime365SeriesID int64
	MyAnimeListID    int64
)

type ExternalNamedLink struct {
	URL  *url.URL
	Name string
}

type Show struct {
	Anime365URL      *url.URL
	PosterURL        *url.URL
	TitleRomaji      string
	TitleRussian     string
	Description      string
	TypeLabel        string
	SeasonLabel      string
	Links            []ExternalNamedLink
	EpisodePreviews  []EpisodePreview
	Genres           []string
	Anime365ID       Anime365SeriesID
	MyAnimeListID    MyAnimeListID
	MyAnimeListScore float64
	Year             int
	IsOngoing        bool
}

type EpisodePreview struct {
	Anime365ID    episode.Anime365EpisodeID
	EpisodeNumber int64
	IsTrailer     bool
	IsSpecial     bool
}

func NewShow(series anime365client.Series) (Show, error) {
	showEntity := Show{
		Anime365ID:    Anime365SeriesID(series.ID),
		TitleRomaji:   strings.TrimSpace(series.Titles.Romaji),
		TitleRussian:  strings.TrimSpace(series.Titles.Ru),
		MyAnimeListID: MyAnimeListID(series.MyAnimeListID),
		Genres:        make([]string, 0, len(series.Genres)),
		IsOngoing:     series.IsAiring == 1,
		Year:          series.Year,
		Links:         make([]ExternalNamedLink, 0, len(series.Links)),
		SeasonLabel:   series.Season,
		TypeLabel:     series.TypeTitle,
	}

	anime365URL, err := url.Parse(series.URL)
	if err != nil {
		return Show{}, fmt.Errorf("failed to parse series anime 365 url: %w", err)
	}

	showEntity.Anime365URL = anime365URL

	if myAnimeListScore, err := strconv.ParseFloat(series.MyAnimeListScore, 64); err == nil && myAnimeListScore > 0 {
		showEntity.MyAnimeListScore = myAnimeListScore
	}

	showEntity.EpisodePreviews = make([]EpisodePreview, 0, len(series.Episodes))

	if series.PosterURL != "" {
		posterURL, err := url.Parse(series.PosterURL)
		if err != nil {
			return Show{}, fmt.Errorf("parsing poster URL: %w", err)
		}

		showEntity.PosterURL = posterURL
	}

	for _, descriptionDTO := range series.Descriptions {
		showEntity.Description = strings.TrimSpace(descriptionDTO.Value)

		break
	}

	for _, genreDTO := range series.Genres {
		showEntity.Genres = append(showEntity.Genres, genreDTO.Title)
	}

	for _, linkDTO := range series.Links {
		u, err := url.Parse(linkDTO.URL)
		if err != nil {
			continue
		}

		showEntity.Links = append(showEntity.Links, ExternalNamedLink{
			Name: strings.TrimSpace(linkDTO.Title),
			URL:  u,
		})
	}

	for _, episodeDTO := range series.Episodes {
		preview := EpisodePreview{
			Anime365ID: episode.Anime365EpisodeID(episodeDTO.ID),
			IsTrailer:  episodeDTO.EpisodeType == "preview",
		}

		episodeNumber, err := strconv.ParseInt(episodeDTO.EpisodeInt, 10, 64)
		if err != nil || episodeNumber <= 0 {
			preview.IsSpecial = true
		} else {
			preview.EpisodeNumber = episodeNumber
		}

		showEntity.EpisodePreviews = append(showEntity.EpisodePreviews, preview)
	}

	return showEntity, nil
}
