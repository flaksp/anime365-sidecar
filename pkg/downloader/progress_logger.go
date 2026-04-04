package downloader

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/flaksp/anime365-sidecar/pkg/filesize"
)

type concurrentProgressLogger struct {
	startTime         time.Time
	lastPrint         time.Time
	ctx               context.Context
	logger            *slog.Logger
	totalBytes        int64
	totalWrittenBytes int64
	logFrequency      time.Duration
	mu                sync.Mutex
}

func newConcurrentProgressLogger(
	ctx context.Context,
	totalBytes int64,
	logger *slog.Logger,
) *concurrentProgressLogger {
	return &concurrentProgressLogger{
		ctx:          ctx,
		totalBytes:   totalBytes,
		startTime:    time.Now(),
		logger:       logger,
		logFrequency: 10 * time.Second,
	}
}

func (p *concurrentProgressLogger) Add(writtenBytes int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.totalWrittenBytes += writtenBytes

	if time.Since(p.lastPrint) > p.logFrequency {
		var (
			percentageFormatted string
			totalSizeFormatted  string
		)

		if p.totalBytes > 0 {
			percentageFormatted = strconv.FormatFloat(
				float64(p.totalWrittenBytes)/float64(p.totalBytes)*100,
				'f',
				2,
				64,
			)
			totalSizeFormatted = filesize.Format(p.totalBytes)
		} else {
			percentageFormatted = "???"
			totalSizeFormatted = "???"
		}

		elapsedSeconds := time.Since(p.startTime).Seconds()
		bytesPerSecond := float64(p.totalWrittenBytes) / elapsedSeconds

		p.logger.DebugContext(p.ctx,
			"Download progress",
			slog.String("percentage", percentageFormatted),
			slog.String("downloaded", filesize.Format(p.totalWrittenBytes)),
			slog.String("total", totalSizeFormatted),
			slog.String("speed", filesize.Format(int64(bytesPerSecond))),
		)

		p.lastPrint = time.Now()
	}
}
