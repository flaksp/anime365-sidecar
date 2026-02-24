package anime365client

import (
	"fmt"
	"net/url"
)

func FormatSeriesURL(baseURL *url.URL, seriesID int64) *url.URL {
	return baseURL.JoinPath(fmt.Sprintf("/catalog/%d", seriesID))
}

func FormatEpisodeURL(baseURL *url.URL, seriesID int64, episodeID int64) *url.URL {
	return baseURL.JoinPath(fmt.Sprintf("/catalog/%d/%d", seriesID, episodeID))
}

func FormatTranslationURL(baseURL *url.URL, seriesID int64, episodeID int64, translationID int64) *url.URL {
	return baseURL.JoinPath(fmt.Sprintf("/catalog/%d/%d/%d", seriesID, episodeID, translationID))
}
