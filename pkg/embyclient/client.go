package embyclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func New(
	baseURL *url.URL,
	httpClient *http.Client,
	timeout time.Duration,
	logger *slog.Logger,
	apiKey string,
) *Client {
	return &Client{
		BaseURL:    baseURL,
		httpClient: httpClient,
		timeout:    timeout,
		logger:     logger,
		apiKey:     apiKey,
	}
}

type Client struct {
	BaseURL    *url.URL
	httpClient *http.Client
	logger     *slog.Logger
	apiKey     string
	timeout    time.Duration
}

func (c *Client) sendGETRequestToAPI(ctx context.Context, endpoint string, queryParams url.Values, response any) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	requestURL := c.BaseURL.JoinPath(endpoint)
	requestURL.RawQuery = queryParams.Encode()
	// Emby API uses URL path decoding when decoding query parameters, so we should replace + with %20
	requestURL.RawQuery = strings.ReplaceAll(requestURL.RawQuery, "+", "%20")

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return fmt.Errorf("creating http request with context: %w", err)
	}

	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("X-Emby-Token", c.apiKey)

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("sending http request: %w", err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			c.logger.WarnContext(ctx, "Emby API response body closed unexpectedly", slog.String("error", err.Error()))
		}
	}(httpResponse.Body)

	responseBodyBytes, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if httpResponse.StatusCode >= 400 {
		c.logger.WarnContext(
			ctx,
			"Unexpected response from Emby API",
			slog.String("method", httpRequest.Method),
			slog.String("url", httpRequest.URL.String()),
			slog.String("status", httpResponse.Status),
			slog.String("response_body", string(responseBodyBytes)),
		)
	}

	if response != nil {
		if err := json.Unmarshal(responseBodyBytes, response); err != nil {
			return fmt.Errorf("unmarshaling response data: %w", err)
		}
	}

	return nil
}

func (c *Client) sendWriteRequestToAPI(
	ctx context.Context,
	httpMethod string,
	endpoint string,
	queryParams url.Values,
	body any,
	response any,
) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	requestURL := c.BaseURL.JoinPath(endpoint)
	requestURL.RawQuery = queryParams.Encode()
	// Emby API uses URL path decoding when decoding query parameters, so we should replace + with %20
	requestURL.RawQuery = strings.ReplaceAll(requestURL.RawQuery, "+", "%20")

	var (
		requestBodyBytes  []byte
		requestBodyReader io.Reader
	)

	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}

		requestBodyBytes = bodyBytes
		requestBodyReader = bytes.NewReader(bodyBytes)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, httpMethod, requestURL.String(), requestBodyReader)
	if err != nil {
		return fmt.Errorf("creating http request with context: %w", err)
	}

	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("X-Emby-Token", c.apiKey)

	if body != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("sending http request: %w", err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			c.logger.WarnContext(ctx, "Emby API response body closed unexpectedly", slog.String("error", err.Error()))
		}
	}(httpResponse.Body)

	responseBodyBytes, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if httpResponse.StatusCode >= 400 {
		c.logger.WarnContext(
			ctx,
			"Unexpected response from Emby API",
			slog.String("method", httpRequest.Method),
			slog.String("url", httpRequest.URL.String()),
			slog.String("status", httpResponse.Status),
			slog.String("request_body", string(requestBodyBytes)),
			slog.String("response_body", string(responseBodyBytes)),
		)
	}

	if response != nil {
		if err := json.Unmarshal(responseBodyBytes, response); err != nil {
			return fmt.Errorf("unmarshaling response data: %w", err)
		}
	}

	return nil
}
