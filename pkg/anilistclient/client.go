package anilistclient

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
	"time"

	"golang.org/x/time/rate"
)

func New(
	baseURL *url.URL,
	httpClient *http.Client,
	timeout time.Duration,
	logger *slog.Logger,
	rateLimiterPerMinute *rate.Limiter,
) *Client {
	return &Client{
		BaseURL:              baseURL,
		httpClient:           httpClient,
		timeout:              timeout,
		logger:               logger,
		rateLimiterPerMinute: rateLimiterPerMinute,
	}
}

type Client struct {
	BaseURL              *url.URL
	httpClient           *http.Client
	logger               *slog.Logger
	rateLimiterPerMinute *rate.Limiter
	timeout              time.Duration
}

func (c *Client) GetMedia(
	ctx context.Context,
	idMal int64,
) (*Media, error) {
	var response GetMediaResponse

	err := c.sendRequestToGraphQL(ctx, graphQLRequest{
		OperationName: "GetMedia",
		Query: `query GetMedia($idMal: Int) {
	Media (idMal: $idMal) {
		bannerImage
	}
}`,
		Variables: map[string]any{
			"idMal": idMal,
		},
	}, &response)
	if err != nil {
		return nil, err
	}

	return response.Media, nil
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

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	requestURL := c.BaseURL

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
		if err := Body.Close(); err != nil {
			c.logger.WarnContext(
				ctx,
				"Anilist API response body closed unexpectedly",
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
			"Unexpected response from Anilist API",
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
			"Error unmarshalling Anilist API response",
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
			"Error unmarshalling Anilist API response",
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
			"Error unmarshalling Anilist API response",
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
