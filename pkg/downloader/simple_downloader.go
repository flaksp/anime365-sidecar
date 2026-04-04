package downloader

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/flaksp/anime365-sidecar/pkg/filesize"
)

func NewSimpleDownloader(
	httpClient *http.Client,
	logger *slog.Logger,
) *SimpleDownloader {
	return &SimpleDownloader{
		httpClient: httpClient,
		logger:     logger,
	}
}

// SimpleDownloader streams response from HTTP to a file
type SimpleDownloader struct {
	httpClient *http.Client
	logger     *slog.Logger
}

func (d *SimpleDownloader) Download(ctx context.Context, fileURL *url.URL, destinationFile *os.File) error {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL.String(), nil)
	if err != nil {
		return fmt.Errorf("creating http request with context: %w", err)
	}

	httpResponse, err := d.httpClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("sending http request: %w", err)
	}

	defer func(httpResponseBody io.ReadCloser) {
		err := httpResponseBody.Close()
		if err != nil {
			d.logger.WarnContext(ctx, "Closing HTTP response body stream error", slog.String("error", err.Error()))
		}
	}(httpResponse.Body)

	if httpResponse.ContentLength > 0 {
		err = destinationFile.Truncate(httpResponse.ContentLength)
		if err != nil {
			return fmt.Errorf("truncating destination file: %w", err)
		}
	}

	d.logger.InfoContext(
		ctx,
		"Starting simple file download",
		slog.String("file_size", filesize.Format(httpResponse.ContentLength)),
		slog.String("file_url", fileURL.String()),
	)

	progressLogger := newConcurrentProgressLogger(
		ctx,
		httpResponse.ContentLength,
		d.logger,
	)

	buffer := make([]byte, 32*1024)

	offset := int64(0)

	for {
		readBytes, err := httpResponse.Body.Read(buffer)

		if readBytes > 0 {
			_, writeErr := destinationFile.WriteAt(buffer[:readBytes], offset)
			if writeErr != nil {
				return writeErr
			}

			offset += int64(readBytes)

			progressLogger.Add(int64(readBytes))
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}
	}

	return nil
}
