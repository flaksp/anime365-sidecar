package downloader

import (
	"context"
	"errors"
	"fmt"
	"net/url"
)

func NewSmartDownloader(
	simpleDownloader *SimpleDownloader,
	chunkedDownloader *ChunkedDownloader,
) *SmartDownloader {
	return &SmartDownloader{
		simpleDownloader:  simpleDownloader,
		chunkedDownloader: chunkedDownloader,
	}
}

type SmartDownloader struct {
	simpleDownloader  *SimpleDownloader
	chunkedDownloader *ChunkedDownloader
}

func (d *SmartDownloader) Download(ctx context.Context, fileURL *url.URL, destinationFilePath string) error {
	err := d.chunkedDownloader.Download(ctx, fileURL, destinationFilePath)
	switch {
	case errors.Is(err, ErrRangeRequestsNotSupported):
		if err := d.simpleDownloader.Download(ctx, fileURL, destinationFilePath); err != nil {
			return fmt.Errorf("failed to download a file using simple downloader: %w", err)
		}

	case err != nil:
		return fmt.Errorf("failed to download a file using chunked downloader: %w", err)
	}

	return nil
}
