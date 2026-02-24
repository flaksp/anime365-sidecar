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
) *Client {
	return &Client{
		BaseURL:    baseURL,
		httpClient: httpClient,
		timeout:    timeout,
		logger:     logger,
	}
}

type Client struct {
	BaseURL    *url.URL
	httpClient *http.Client
	logger     *slog.Logger
	timeout    time.Duration
}

func (c *Client) GetSeries(ctx context.Context, seriesID int64) (Series, error) {
	var response Series

	err := c.sendRequestToAPI(ctx, fmt.Sprintf("/api/series/%d", seriesID), nil, &response)

	return response, err
}

func (c *Client) GetEpisode(ctx context.Context, episodeID int64) (Episode, error) {
	var response Episode

	err := c.sendRequestToAPI(ctx, fmt.Sprintf("/api/episodes/%d", episodeID), nil, &response)

	return response, err
}

func (c *Client) GetTranslation(ctx context.Context, translationID int64) (Translation, error) {
	var response Translation

	err := c.sendRequestToAPI(ctx, fmt.Sprintf("/api/translations/%d", translationID), nil, &response)

	return response, err
}

func (c *Client) GetTranslationEmbed(ctx context.Context, translationID int64) (TranslationEmbed, error) {
	var response TranslationEmbed

	err := c.sendRequestToAPI(ctx, fmt.Sprintf("/api/translations/embed/%d", translationID), nil, &response)

	return response, err
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

func (c *Client) sendRequestToAPI(ctx context.Context, endpoint string, queryParams url.Values, response any) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	requestURL := c.BaseURL.JoinPath(endpoint)
	requestURL.RawQuery = queryParams.Encode()

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return fmt.Errorf("creating http request with context: %w", err)
	}

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("sending http request: %w", err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			c.logger.WarnContext(
				ctx,
				"Anime 365 API response body closed unexpectedly",
				slog.String("error", err.Error()),
			)
		}
	}(httpResponse.Body)

	responseBodyBytes, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if httpResponse.StatusCode >= 400 {
		c.logger.WarnContext(
			ctx,
			"Unexpected response from Anime 365 API",
			slog.String("method", httpRequest.Method),
			slog.String("url", httpRequest.URL.String()),
			slog.String("status", httpResponse.Status),
			slog.String("response_body", string(responseBodyBytes)),
		)
	}

	var apiError APIError
	if err := json.Unmarshal(responseBodyBytes, &apiError); err == nil && apiError.Code != 0 {
		return &apiError
	}

	var wrappedResponse struct {
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(responseBodyBytes, &wrappedResponse); err != nil {
		return fmt.Errorf("unmarshaling wrapped response: %w", err)
	}

	if len(wrappedResponse.Data) == 0 {
		return errors.New("empty data field in response")
	}

	if err := json.Unmarshal(wrappedResponse.Data, response); err != nil {
		return fmt.Errorf("unmarshaling response data: %w", err)
	}

	return nil
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
			StatusCode: httpResponse.StatusCode,
			Status:     httpResponse.Status,
		}
	}

	return responseBodyBytes, nil
}

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
			slog.String("request_body", formParams.Encode()),
			slog.String("response_body", string(responseBodyBytes)),
		)

		return nil, WebError{
			StatusCode: httpResponse.StatusCode,
			Status:     httpResponse.Status,
		}
	}

	return responseBodyBytes, nil
}
