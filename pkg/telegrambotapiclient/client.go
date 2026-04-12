package telegrambotapiclient

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
)

func New(
	baseURL *url.URL,
	botAPIToken string,
	httpClient *http.Client,
	timeout time.Duration,
	logger *slog.Logger,
) *Client {
	return &Client{
		BaseURL:     baseURL,
		botAPIToken: botAPIToken,
		httpClient:  httpClient,
		timeout:     timeout,
		logger:      logger,
	}
}

type Client struct {
	BaseURL     *url.URL
	httpClient  *http.Client
	logger      *slog.Logger
	botAPIToken string
	timeout     time.Duration
}

type SendMessageOptionalParams struct {
	LinkPreviewOptions *LinkPreviewOptions
	ParseMode          string
}

func (c *Client) SendMessage(
	ctx context.Context,
	chatID any,
	text string,
	optionalParams *SendMessageOptionalParams,
) (Message, error) {
	var response Message

	params := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}

	if optionalParams != nil {
		if optionalParams.ParseMode != "" {
			params["parse_mode"] = optionalParams.ParseMode
		}

		if optionalParams.LinkPreviewOptions != nil {
			linkPreviewOptionsBytes, err := json.Marshal(optionalParams.LinkPreviewOptions)
			if err != nil {
				return response, fmt.Errorf("failed to marshal link preview options: %w", err)
			}

			params["link_preview_options"] = string(linkPreviewOptionsBytes)
		}
	}

	err := c.sendRequestToAPI(ctx, "sendMessage", params, &response)

	return response, err
}

func (c *Client) sendRequestToAPI(ctx context.Context, apiMethod string, params map[string]any, response any) error {
	httpRequestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	requestURL := c.BaseURL.JoinPath("bot"+c.botAPIToken, apiMethod)

	requestBodyBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshalling request body: %w", err)
	}

	requestBodyReader := bytes.NewReader(requestBodyBytes)

	httpRequest, err := http.NewRequestWithContext(
		httpRequestCtx,
		http.MethodPost,
		requestURL.String(),
		requestBodyReader,
	)
	if err != nil {
		return fmt.Errorf("creating http request with context: %w", err)
	}

	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("sending http request: %w", err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			c.logger.WarnContext(
				httpRequestCtx,
				"Telegram Bot API response body closed unexpectedly",
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
			"Unexpected response from Telegram Bot API",
			slog.String("method", httpRequest.Method),
			slog.String("url", httpRequest.URL.String()),
			slog.String("status", httpResponse.Status),
			slog.String("response_body", string(responseBodyBytes)),
		)
	}

	var wrappedResponse struct {
		Result json.RawMessage `json:"result"`
	}

	if err := json.Unmarshal(responseBodyBytes, &wrappedResponse); err != nil {
		return fmt.Errorf("unmarshaling wrapped response: %w", err)
	}

	if len(wrappedResponse.Result) == 0 {
		return errors.New("empty result field in response")
	}

	if err := json.Unmarshal(wrappedResponse.Result, response); err != nil {
		return fmt.Errorf("unmarshaling response data: %w", err)
	}

	return nil
}
