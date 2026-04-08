package jikanclient

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
	"time"

	"golang.org/x/time/rate"
)

func New(
	baseURL *url.URL,
	httpClient *http.Client,
	timeout time.Duration,
	logger *slog.Logger,
	rateLimiterPerSecond *rate.Limiter,
	rateLimiterPerMinute *rate.Limiter,
) *Client {
	return &Client{
		BaseURL:              baseURL,
		httpClient:           httpClient,
		timeout:              timeout,
		logger:               logger,
		rateLimiterPerSecond: rateLimiterPerSecond,
		rateLimiterPerMinute: rateLimiterPerMinute,
	}
}

type Client struct {
	BaseURL              *url.URL
	httpClient           *http.Client
	logger               *slog.Logger
	rateLimiterPerSecond *rate.Limiter
	rateLimiterPerMinute *rate.Limiter
	timeout              time.Duration
}

func (c *Client) GetAnimeEpisodes(
	ctx context.Context,
	id int64,
	page int64,
) ([]AnimeEpisodeListingItem, error) {
	var response []AnimeEpisodeListingItem

	queryParams := url.Values{}

	if page > 0 {
		queryParams.Add("page", strconv.FormatInt(page, 10))
	}

	err := c.sendRequestToAPI(ctx, fmt.Sprintf("/v4/anime/%d/episodes", id), queryParams, &response)

	return response, err
}

func (c *Client) GetAnimeEpisodeByID(
	ctx context.Context,
	id int64,
	episode int64,
) (AnimeEpisode, error) {
	var response AnimeEpisode

	err := c.sendRequestToAPI(ctx, fmt.Sprintf("/v4/anime/%d/episodes/%d", id, episode), nil, &response)

	return response, err
}

func (c *Client) sendRequestToAPI(ctx context.Context, endpoint string, queryParams url.Values, response any) error {
	if c.rateLimiterPerMinute != nil {
		if err := c.rateLimiterPerMinute.Wait(ctx); err != nil {
			return fmt.Errorf("per-minute rate limit: %w", err)
		}
	}

	if c.rateLimiterPerSecond != nil {
		if err := c.rateLimiterPerSecond.Wait(ctx); err != nil {
			return fmt.Errorf("per-second rate limit: %w", err)
		}
	}

	httpRequestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	requestURL := c.BaseURL.JoinPath(endpoint)
	requestURL.RawQuery = queryParams.Encode()

	httpRequest, err := http.NewRequestWithContext(httpRequestCtx, http.MethodGet, requestURL.String(), nil)
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
				httpRequestCtx,
				"Jikan API response body closed unexpectedly",
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
			httpRequestCtx,
			"Unexpected response from Jikan API",
			slog.String("method", httpRequest.Method),
			slog.String("url", httpRequest.URL.String()),
			slog.String("status", httpResponse.Status),
			slog.String("response_body", string(responseBodyBytes)),
		)
	}

	var apiError APIError
	if err := json.Unmarshal(responseBodyBytes, &apiError); err == nil && apiError.Status != 0 {
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
