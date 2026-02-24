package downloader

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
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

func (d *SmartDownloader) Download(ctx context.Context, fileURL *url.URL, destinationFile *os.File) error {
	err := d.chunkedDownloader.Download(ctx, fileURL, destinationFile)
	switch {
	case errors.Is(err, ErrRangeRequestsNotSupported):
		err = d.simpleDownloader.Download(ctx, fileURL, destinationFile)
		if err != nil {
			return fmt.Errorf("failed to download a file using simple downloader: %w", err)
		}

	case err != nil:
		return fmt.Errorf("failed to download a file using chunked downloader: %w", err)
	}

	return nil
}
