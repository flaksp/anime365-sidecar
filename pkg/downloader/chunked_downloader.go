package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/flaksp/anime365-emby/pkg/filesize"
	"golang.org/x/sync/errgroup"
)

func NewChunkedDownloader(
	httpClient *http.Client,
	logger *slog.Logger,
	chunkSize int64,
	workersCount int,
) *ChunkedDownloader {
	return &ChunkedDownloader{
		httpClient:     httpClient,
		logger:         logger,
		chunkSizeBytes: chunkSize,
		workersCount:   workersCount,
	}
}

type ChunkedDownloader struct {
	httpClient *http.Client
	logger     *slog.Logger

	chunkSizeBytes int64
	workersCount   int
}

var ErrRangeRequestsNotSupported = errors.New("range requests not supported")

func (d *ChunkedDownloader) Download(
	ctx context.Context,
	fileURL *url.URL,
	destinationFile *os.File,
) error {
	fileSizeBytes, supportsRangeRequests, err := d.getMetadata(ctx, fileURL)
	if err != nil {
		return fmt.Errorf("downloading metadata: %w", err)
	}

	if !supportsRangeRequests {
		return ErrRangeRequestsNotSupported
	}

	if fileSizeBytes > 0 {
		err = destinationFile.Truncate(fileSizeBytes)
		if err != nil {
			return fmt.Errorf("truncating destination file: %w", err)
		}
	}

	totalChunks := (fileSizeBytes + d.chunkSizeBytes - 1) / d.chunkSizeBytes

	d.logger.InfoContext(ctx, "Starting chunked file download",
		slog.String("file_size", filesize.Format(fileSizeBytes)),
		slog.Int("workers", d.workersCount),
		slog.Int64("total_chunks", totalChunks),
		slog.String("file_url", fileURL.String()),
	)

	progressLogger := newConcurrentProgressLogger(
		ctx,
		fileSizeBytes,
		d.logger,
	)

	var mu sync.Mutex

	nextChunk := int64(0)

	errGroup, errGroupCtx := errgroup.WithContext(ctx)

	for i := 0; i < d.workersCount; i++ {
		errGroup.Go(func() error {
			for {
				if errGroupCtx.Err() != nil {
					return errGroupCtx.Err()
				}

				mu.Lock()

				if nextChunk >= totalChunks {
					mu.Unlock()

					return nil
				}

				chunkIndex := nextChunk
				nextChunk++

				mu.Unlock()

				chunkStartBytes := chunkIndex * d.chunkSizeBytes
				chunkEndBytes := chunkStartBytes + d.chunkSizeBytes - 1

				if chunkEndBytes >= fileSizeBytes {
					chunkEndBytes = fileSizeBytes - 1
				}

				err := d.downloadChunk(
					errGroupCtx,
					fileURL,
					destinationFile,
					chunkStartBytes,
					chunkEndBytes,
					progressLogger,
				)
				if err != nil {
					return fmt.Errorf(
						"downloading chunk %d (%d-%d): %w",
						chunkIndex,
						chunkStartBytes,
						chunkEndBytes,
						err,
					)
				}
			}
		})
	}

	err = errGroup.Wait()
	if err != nil {
		return fmt.Errorf("downloading chunks: %w", err)
	}

	return nil
}

func (d *ChunkedDownloader) downloadChunk(
	ctx context.Context,
	fileURL *url.URL,
	destinationFile *os.File,
	chunkStartBytes int64,
	chunkEndBytes int64,
	progressLogger *concurrentProgressLogger,
) error {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL.String(), nil)
	if err != nil {
		return err
	}

	httpRequest.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", chunkStartBytes, chunkEndBytes))

	httpResponse, err := d.httpClient.Do(httpRequest)
	if err != nil {
		return err
	}

	defer func(httpResponseBody io.ReadCloser) {
		err := httpResponseBody.Close()
		if err != nil {
			d.logger.WarnContext(ctx, "Closing HTTP response body stream error", slog.String("error", err.Error()))
		}
	}(httpResponse.Body)

	if httpResponse.StatusCode != http.StatusPartialContent {
		return fmt.Errorf(
			"expecting 206 partial content, but unexpected status returned: %s",
			httpResponse.Status)
	}

	buffer := make([]byte, 32*1024)

	offset := chunkStartBytes

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

func (d *ChunkedDownloader) getMetadata(ctx context.Context, fileURL *url.URL) (int64, bool, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodHead, fileURL.String(), nil)
	if err != nil {
		return 0, false, fmt.Errorf("creating http request with context: %w", err)
	}

	httpResponse, err := d.httpClient.Do(httpRequest)
	if err != nil {
		return 0, false, fmt.Errorf("sending http request: %w", err)
	}

	d.logger.DebugContext(ctx, "Getting file metadata", slog.String("file_url", fileURL.String()))

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			d.logger.WarnContext(
				ctx,
				"Closing HTTP response body stream error",
				slog.String("error", err.Error()),
			)
		}
	}(httpResponse.Body)

	return httpResponse.ContentLength, httpResponse.Header.Get("Accept-Ranges") == "bytes", nil
}
