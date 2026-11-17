package anime365client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/flaksp/anime365-sidecar/pkg/anime365client/internal/api"
	"github.com/google/uuid"
)

const (
	cookieNameCSRF    = "csrf"
	formDataFieldCSRF = "csrf"
)

var ErrAuthenticationRequired = errors.New("authentication required")

func New(
	baseURL *url.URL,
	httpClient *http.Client,
	timeout time.Duration,
	logger *slog.Logger,
) (*Client, error) {
	apiClient, err := api.NewClientWithResponses(baseURL.JoinPath("api").String(), api.WithHTTPClient(httpClient),
		api.WithRequestEditorFn(func(_ context.Context, request *http.Request) error {
			request.Header.Set("User-Agent", "anime365-sidecar")

			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("creating Anime365 API client: %w", err)
	}

	return &Client{
		BaseURL:    baseURL,
		httpClient: httpClient,
		timeout:    timeout,
		logger:     logger,
		apiClient:  apiClient,
	}, nil
}

type Client struct {
	apiClient  *api.ClientWithResponses
	BaseURL    *url.URL
	httpClient *http.Client
	logger     *slog.Logger
	timeout    time.Duration
}

func (c *Client) GetSeries(ctx context.Context, seriesID int64) (Series, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	response, err := c.apiClient.GetSeriesWithResponse(ctx, seriesID, &api.GetSeriesParams{
		Fields: strings.Join([]string{
			"id",
			"titles",
			"myAnimeListId",
			"myAnimeListScore",
			"genres",
			"isAiring",
			"year",
			"season",
			"typeTitle",
			"numberOfEpisodes",
			"episodes",
			"posterUrl",
			"descriptions",
		}, ","),
	})
	if err != nil {
		return Series{}, fmt.Errorf("requesting Anime365 API: %w", err)
	}

	var data *Series
	if response.JSON200 != nil {
		data = response.JSON200.Data
	}

	return decodeAPIResponse(response.Body, response.HTTPResponse, data)
}

func (c *Client) GetEpisode(ctx context.Context, episodeID int64) (Episode, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	response, err := c.apiClient.GetEpisodeWithResponse(ctx, episodeID, &api.GetEpisodeParams{
		Fields: strings.Join([]string{
			"id",
			"episodeType",
			"episodeFull",
			"isActive",
			"episodeTitle",
			"episodeInt",
			"translations",
			"firstUploadedDateTime",
		}, ","),
	})
	if err != nil {
		return Episode{}, fmt.Errorf("requesting Anime365 API: %w", err)
	}

	var data *Episode
	if response.JSON200 != nil {
		data = response.JSON200.Data
	}

	return decodeAPIResponse(response.Body, response.HTTPResponse, data)
}

func (c *Client) GetTranslation(ctx context.Context, translationID int64) (Translation, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	response, err := c.apiClient.GetTranslationWithResponse(ctx, translationID, &api.GetTranslationParams{
		Fields: strings.Join([]string{
			"id",
			"authorsList",
			"priority",
			"isActive",
			"typeKind",
			"typeLang",
			"activeDateTime",
		}, ","),
	})
	if err != nil {
		return Translation{}, fmt.Errorf("requesting Anime365 API: %w", err)
	}

	var data *Translation
	if response.JSON200 != nil {
		data = response.JSON200.Data
	}

	return decodeAPIResponse(response.Body, response.HTTPResponse, data)
}

func (c *Client) GetTranslationEmbed(ctx context.Context, translationID int64) (TranslationEmbed, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	response, err := c.apiClient.GetTranslationEmbedWithResponse(ctx, translationID, nil)
	if err != nil {
		return TranslationEmbed{}, fmt.Errorf("requesting Anime365 API: %w", err)
	}

	var data *TranslationEmbed
	if response.JSON200 != nil {
		data = response.JSON200.Data
	}

	return decodeAPIResponse(response.Body, response.HTTPResponse, data)
}

func (c *Client) Login(ctx context.Context, username string, password string) error {
	_, err := c.sendPOSTRequestToWeb(ctx, "/users/login", nil, url.Values{
		"LoginForm[username]": {username},
		"LoginForm[password]": {password},
		"dynpage":             {"1"},
		"yt0":                 {""},
	})

	return err
}

func (c *Client) MarkTranslationAsWatched(ctx context.Context, translationID int64) error {
	_, err := c.sendPOSTRequestToWeb(ctx, fmt.Sprintf("/translations/watched/%d", translationID), nil, nil)

	return err
}

func (c *Client) GetMe(ctx context.Context) (Profile, error) {
	htmlBytes, err := c.sendGETRequestToWeb(ctx, "/users/profile", url.Values{
		"dynpage": {"1"},
	})
	if err != nil {
		return Profile{}, fmt.Errorf("failed to send http request: %w", err)
	}

	html := string(htmlBytes)

	if strings.Contains(html, "Вход по паролю") {
		return Profile{}, ErrAuthenticationRequired
	}

	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return Profile{}, fmt.Errorf("failed to parse html: %w", err)
	}

	// Extract account ID using regex
	re := regexp.MustCompile(`ID аккаунта: (\d+)`)
	matches := re.FindStringSubmatch(html)

	if len(matches) < 2 {
		return Profile{}, errors.New("could not find profile id on page")
	}

	profileID, err := strconv.Atoi(matches[1])
	if err != nil {
		return Profile{}, errors.New("profile id could not be parsed as int")
	}

	nameSel := doc.Find("content .m-small-title")
	if nameSel.Length() == 0 {
		return Profile{}, errors.New("could not find profile name on page")
	}

	return Profile{
		ID:   int64(profileID),
		Name: strings.TrimSpace(nameSel.First().Text()),
	}, nil
}

const (
	AnimeListIDWatching  = "watching"
	AnimeListIDPlanned   = "planned"
	AnimeListIDCompleted = "completed"
	AnimeListIDOnHold    = "onhold"
	AnimeListIDDropped   = "dropped"
)

func (c *Client) GetAnimeList(ctx context.Context, profileID int64, listID string) ([]AnimeListItem, error) {
	htmlBytes, err := c.sendGETRequestToWeb(ctx, fmt.Sprintf("/users/%d/list/%s", profileID, listID), url.Values{
		"dynpage": {"1"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send http request: %w", err)
	}

	html := string(htmlBytes)

	if strings.Contains(html, "Вход или регистрация") || strings.Contains(html, "Вход - Anime 365") {
		return nil, ErrAuthenticationRequired
	}

	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse html: %w", err)
	}

	listItemsSel := doc.Find("div.card.m-animelist-card tr.m-animelist-item")

	res := make([]AnimeListItem, 0, listItemsSel.Length())

	listItemsSel.Each(func(_ int, itemSel *goquery.Selection) {
		seriesIDStr, exists := itemSel.Attr("data-id")
		if !exists {
			c.logger.WarnContext(ctx, "Normalizing anime list entry failure: could not find series ID")

			return
		}

		seriesID, err := strconv.ParseInt(seriesIDStr, 10, 64)
		if err != nil {
			c.logger.WarnContext(ctx,
				"Normalizing anime list entry failure: could parse series ID",
				slog.String("error", err.Error()),
			)

			return
		}

		episodesString := strings.TrimSpace(itemSel.Find("td[data-name=\"episodes\"]").First().Text())
		if episodesString == "" {
			c.logger.WarnContext(ctx, "Normalizing anime list entry failure: could not find episodes string")

			return
		}

		splittedEpisodesString := strings.SplitN(episodesString, " / ", 2)
		if len(splittedEpisodesString) != 2 {
			c.logger.WarnContext(ctx, "Normalizing anime list entry failure: episodes string is not valid")

			return
		}

		episodesWatched, err := strconv.ParseInt(strings.TrimSpace(splittedEpisodesString[0]), 10, 64)
		if err != nil {
			c.logger.WarnContext(ctx,
				"Normalizing anime list entry failure: could parse episodes watched count",
				slog.String("error", err.Error()),
			)

			return
		}

		res = append(res, AnimeListItem{
			ID:              seriesID,
			EpisodesWatched: episodesWatched,
		})
	})

	return res, nil
}

func (c *Client) GetSubtitles(ctx context.Context, path string) ([]byte, error) {
	subtitlesBytes, err := c.sendGETRequestToWeb(ctx, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to send http request: %w", err)
	}

	return subtitlesBytes, nil
}

func decodeAPIResponse[T any](body []byte, response *http.Response, data *T) (T, error) {
	var (
		zero         T
		wrappedError api.Error
	)
	if err := json.Unmarshal(body, &wrappedError); err == nil && wrappedError.Error.Code != 0 {
		return zero, &APIError{Code: wrappedError.Error.Code, Message: wrappedError.Error.Message}
	}
	// Preserve support for the legacy unwrapped API error format.
	var apiError APIError
	if err := json.Unmarshal(body, &apiError); err == nil && apiError.Code != 0 {
		return zero, &apiError
	}

	if response.StatusCode != http.StatusOK {
		return zero, &APIError{Code: response.StatusCode, Message: response.Status}
	}

	if data == nil {
		return zero, errors.New("empty data field in Anime365 API response")
	}

	return *data, nil
}

func (c *Client) sendGETRequestToWeb(ctx context.Context, endpoint string, queryParams url.Values) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	requestURL := c.BaseURL.JoinPath(endpoint)
	requestURL.RawQuery = queryParams.Encode()

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating http request with context: %w", err)
	}

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("sending http request: %w", err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			c.logger.WarnContext(
				ctx,
				"Anime 365 web response body closed unexpectedly",
				slog.String("error", err.Error()),
			)
		}
	}(httpResponse.Body)

	responseBodyBytes, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if httpResponse.StatusCode >= 400 {
		c.logger.WarnContext(
			ctx,
			"Unexpected response from Anime 365 website",
			slog.String("method", httpRequest.Method),
			slog.String("url", httpRequest.URL.String()),
			slog.String("status", httpResponse.Status),
			slog.String("response_body", string(responseBodyBytes)),
		)

		return nil, WebError{
			Status: httpResponse.Status,
		}
	}

	return responseBodyBytes, nil
}

// nolint:unparam
func (c *Client) sendPOSTRequestToWeb(
	ctx context.Context,
	endpoint string,
	queryParams url.Values,
	formParams url.Values,
) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	requestURL := c.BaseURL.JoinPath(endpoint)
	requestURL.RawQuery = queryParams.Encode()

	var csrfTokenFromCookie string

	randomCsrfToken := uuid.New()

	for _, cookie := range c.httpClient.Jar.Cookies(requestURL) {
		if cookie.Name == cookieNameCSRF {
			csrfTokenFromCookie = cookie.Value

			break
		}
	}

	if formParams == nil {
		formParams = url.Values{}
	}

	if csrfTokenFromCookie != "" {
		formParams.Add(formDataFieldCSRF, csrfTokenFromCookie)
	} else {
		c.httpClient.Jar.SetCookies(requestURL, []*http.Cookie{
			{
				Name:   cookieNameCSRF,
				Value:  randomCsrfToken.String(),
				Domain: c.BaseURL.Host,
				Path:   "/",
			},
		})

		formParams.Add(formDataFieldCSRF, randomCsrfToken.String())
	}

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		requestURL.String(),
		strings.NewReader(formParams.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("creating http request with context: %w", err)
	}

	httpRequest.Header.Set("Accept", "text/html")
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("sending http request: %w", err)
	}

	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			c.logger.WarnContext(
				ctx,
				"Anime 365 web response body closed unexpectedly",
				slog.String("error", err.Error()),
			)
		}
	}(httpResponse.Body)

	responseBodyBytes, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if httpResponse.StatusCode >= 400 {
		c.logger.WarnContext(
			ctx,
			"Unexpected response from Anime 365 website",
			slog.String("method", httpRequest.Method),
			slog.String("url", httpRequest.URL.String()),
			slog.String("status", httpResponse.Status),
			slog.String("request_body", formParams.Encode()),
			slog.String("response_body", string(responseBodyBytes)),
		)

		return nil, WebError{
			Status: httpResponse.Status,
		}
	}

	return responseBodyBytes, nil
}
