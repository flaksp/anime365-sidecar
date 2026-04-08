package shikimoriclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

const QueryAnimesMaxLimit = 50

func (c *Client) GetAnimes(
	ctx context.Context,
	ids []int64,
) ([]Anime, error) {
	idsStrings := make([]string, 0, len(ids))
	for _, id := range ids {
		idsStrings = append(idsStrings, strconv.FormatInt(id, 10))
	}

	var response GetAnimesResponse

	err := c.sendRequestToGraphQL(ctx, graphQLRequest{
		OperationName: "GetAnimes",
		Query: `query GetAnimes($ids: String, $limit: PositiveInt, $censored: Boolean) {
  animes(ids: $ids, limit: $limit, censored: $censored) {
    id
    rating
    duration
    status
    airedOn { day, month, year }
    releasedOn { day, month, year }
    studios { name }
    screenshots { id originalUrl }
    personRoles { rolesEn, person { name } }
  }
}`,
		Variables: map[string]any{
			"ids":      strings.Join(idsStrings, ","),
			"limit":    len(idsStrings),
			"censored": false,
		},
	}, &response)
	if err != nil {
		return nil, err
	}

	return response.Animes, nil
}

func (c *Client) sendRequestToGraphQL(
	ctx context.Context,
	request graphQLRequest,
	response any,
) error {
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

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	requestURL := c.BaseURL.JoinPath("/api/graphql")

	requestBodyBytes, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshalling request body: %w", err)
	}

	requestBodyReader := bytes.NewReader(requestBodyBytes)

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), requestBodyReader)
	if err != nil {
		return fmt.Errorf("creating http request with context: %w", err)
	}

	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("User-Agent", "anime365-media-server-sidecar")

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("sending http request: %w", err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			c.logger.WarnContext(
				ctx,
				"Shikimori API response body closed unexpectedly",
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
			"Unexpected response from Shikimori API",
			slog.String("method", httpRequest.Method),
			slog.String("url", httpRequest.URL.String()),
			slog.String("status", httpResponse.Status),
			slog.String("request_body", string(requestBodyBytes)),
			slog.String("response_body", string(responseBodyBytes)),
		)
	}

	var wrappedResponse struct {
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(responseBodyBytes, &wrappedResponse); err != nil {
		c.logger.WarnContext(
			ctx,
			"Error unmarshalling Shikimori API response",
			slog.String("method", httpRequest.Method),
			slog.String("url", httpRequest.URL.String()),
			slog.String("status", httpResponse.Status),
			slog.String("request_body", string(requestBodyBytes)),
			slog.String("response_body", string(responseBodyBytes)),
		)

		return fmt.Errorf("unmarshaling wrapped response: %w", err)
	}

	if len(wrappedResponse.Data) == 0 {
		c.logger.WarnContext(
			ctx,
			"Error unmarshalling Shikimori API response",
			slog.String("method", httpRequest.Method),
			slog.String("url", httpRequest.URL.String()),
			slog.String("status", httpResponse.Status),
			slog.String("request_body", string(requestBodyBytes)),
			slog.String("response_body", string(responseBodyBytes)),
		)

		return errors.New("empty data field in response")
	}

	if err := json.Unmarshal(wrappedResponse.Data, response); err != nil {
		c.logger.WarnContext(
			ctx,
			"Error unmarshalling Shikimori API response",
			slog.String("method", httpRequest.Method),
			slog.String("url", httpRequest.URL.String()),
			slog.String("status", httpResponse.Status),
			slog.String("request_body", string(requestBodyBytes)),
			slog.String("response_body", string(responseBodyBytes)),
		)

		return fmt.Errorf("unmarshaling response data: %w", err)
	}

	return nil
}
