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
	"time"

	"github.com/flaksp/anime365-sidecar/pkg/filesize"
	"golang.org/x/sync/errgroup"
)

func NewChunkedDownloader(
	httpClient *http.Client,
	logger *slog.Logger,
	chunkSize int64,
	workersCount int,
	maxChunkRetries int,
	retryBaseDelay time.Duration,
) *ChunkedDownloader {
	return &ChunkedDownloader{
		httpClient:      httpClient,
		logger:          logger,
		chunkSizeBytes:  chunkSize,
		workersCount:    workersCount,
		maxChunkRetries: maxChunkRetries,
		retryBaseDelay:  retryBaseDelay,
	}
}

type ChunkedDownloader struct {
	httpClient *http.Client
	logger     *slog.Logger

	chunkSizeBytes  int64
	workersCount    int
	maxChunkRetries int
	retryBaseDelay  time.Duration
}

var ErrRangeRequestsNotSupported = errors.New("range requests not supported")

func (d *ChunkedDownloader) Download(
	ctx context.Context,
	fileURL *url.URL,
	destinationFilePath string,
) error {
	fileSizeBytes, supportsRangeRequests, err := d.getMetadata(ctx, fileURL)
	if err != nil {
		return fmt.Errorf("downloading metadata: %w", err)
	}

	if !supportsRangeRequests {
		return ErrRangeRequestsNotSupported
	}

	destinationFile, err := os.OpenFile(destinationFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("opening destination file: %w", err)
	}

	defer func() {
		if err := destinationFile.Close(); err != nil {
			d.logger.WarnContext(
				ctx,
				"Closing destination file in chunked downloader error",
				slog.String("error", err.Error()),
				slog.String("file_path", destinationFilePath),
			)
		}
	}()

	if fileSizeBytes > 0 {
		if err := destinationFile.Truncate(fileSizeBytes); err != nil {
			return fmt.Errorf("truncating destination file: %w", err)
		}
	}

	totalChunks := (fileSizeBytes + d.chunkSizeBytes - 1) / d.chunkSizeBytes

	d.logger.InfoContext(ctx, "Starting chunked file download",
		slog.String("file_size", filesize.Format(fileSizeBytes)),
		slog.Int("workers", d.workersCount),
		slog.Int64("chunk_size", d.chunkSizeBytes),
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

				if err := d.downloadChunkWithRetry(
					errGroupCtx,
					fileURL,
					destinationFile,
					chunkIndex,
					chunkStartBytes,
					chunkEndBytes,
					progressLogger,
				); err != nil {
					return err
				}
			}
		})
	}

	if err := errGroup.Wait(); err != nil {
		return fmt.Errorf("downloading chunks: %w", err)
	}

	if err := destinationFile.Sync(); err != nil {
		return fmt.Errorf("sync destination file: %w", err)
	}

	return nil
}

func (d *ChunkedDownloader) downloadChunkWithRetry(
	ctx context.Context,
	fileURL *url.URL,
	destinationFile *os.File,
	chunkIndex int64,
	chunkStartBytes int64,
	chunkEndBytes int64,
	progressLogger *concurrentProgressLogger,
) error {
	var lastErr error

	for attempt := range d.maxChunkRetries + 1 {
		writtenBytes, err := d.downloadChunk(
			ctx,
			fileURL,
			destinationFile,
			chunkStartBytes,
			chunkEndBytes,
		)
		progressLogger.Add(int64(writtenBytes))

		if err == nil {
			return nil
		}

		lastErr = err

		if attempt == d.maxChunkRetries {
			break
		}

		// Undo partial progress before retrying — the chunk will be
		// re-downloaded in full from chunkStartBytes on the next attempt
		progressLogger.Add(-int64(writtenBytes))

		// exponential backoff: 1 sec, 2 sec, 4 sec, 8 sec..
		delay := d.retryBaseDelay * (1 << attempt)

		d.logger.WarnContext(ctx, "Failed to download chunk, retrying after exponential backoff...",
			slog.String("error", lastErr.Error()),
			slog.Int64("chunk_index", chunkIndex),
			slog.Int64("chunk_start_bytes", chunkStartBytes),
			slog.Int64("chunk_end_bytes", chunkEndBytes),
			slog.Int("attempt", attempt),
			slog.Int("attempts_max", d.maxChunkRetries),
			slog.Duration("delay", delay),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return fmt.Errorf(
		"chunk %d (%d-%d) failed after %d attempts: %w",
		chunkIndex,
		chunkStartBytes,
		chunkEndBytes,
		d.maxChunkRetries+1,
		lastErr,
	)
}

func (d *ChunkedDownloader) downloadChunk(
	ctx context.Context,
	fileURL *url.URL,
	destinationFile *os.File,
	chunkStartBytes int64,
	chunkEndBytes int64,
) (int, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL.String(), nil)
	if err != nil {
		return 0, err
	}

	httpRequest.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", chunkStartBytes, chunkEndBytes))

	httpResponse, err := d.httpClient.Do(httpRequest)
	if err != nil {
		return 0, err
	}

	defer func(httpResponseBody io.ReadCloser) {
		if err := httpResponseBody.Close(); err != nil {
			d.logger.WarnContext(ctx, "Closing HTTP response body stream error", slog.String("error", err.Error()))
		}
	}(httpResponse.Body)

	if httpResponse.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf(
			"expecting 206 partial content, but unexpected status returned: %s",
			httpResponse.Status,
		)
	}

	buffer := make([]byte, 32*1024)
	offset := chunkStartBytes

	var writtenBytes int

	for {
		readBytes, err := httpResponse.Body.Read(buffer)

		if readBytes > 0 {
			writtenBytesIter, writeErr := destinationFile.WriteAt(buffer[:readBytes], offset)

			writtenBytes += writtenBytesIter
			if writeErr != nil {
				return writtenBytes, writeErr
			}

			offset += int64(readBytes)
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return writtenBytes, err
		}
	}

	return writtenBytes, nil
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
		if err := Body.Close(); err != nil {
			d.logger.WarnContext(ctx, "Closing HTTP response body stream error", slog.String("error", err.Error()))
		}
	}(httpResponse.Body)

	return httpResponse.ContentLength, httpResponse.Header.Get("Accept-Ranges") == "bytes", nil
}
