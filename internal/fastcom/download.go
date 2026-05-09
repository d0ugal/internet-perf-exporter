package fastcom

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// runDownloadTest performs download speed test
func (c *Client) runDownloadTest(ctx context.Context, maxTime time.Duration) (float64, error) {
	ctx, span := otel.Tracer("fastcom").Start(ctx, "fastcom.downloadTest")
	defer span.End()

	span.SetAttributes(
		attribute.String("download.max_time", maxTime.String()),
	)
	if len(c.urls) == 0 {
		err := fmt.Errorf("no test URLs available")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var maxSpeedBps float64

	// totalBytes is shared across all download goroutines so we compute
	// AGGREGATE throughput. The previous version kept a per-goroutine
	// totalBytes and reported max(per-goroutine speed) — undercounting
	// real bandwidth by a factor of N (8 by default) on multi-connection
	// downloads.
	var totalBytes atomic.Int64

	done := make(chan struct{})

	// Single global start so every measurement uses the same elapsed.
	startTime := time.Now()

	// Start multiple concurrent downloads
	connections := 8
	if connections > len(c.urls) {
		connections = len(c.urls)
	}

	for i := 0; i < connections; i++ {
		wg.Add(1)
		go func(urlIndex int) {
			defer wg.Done()
			testURL := c.urls[urlIndex%len(c.urls)]

			for time.Since(startTime) < maxTime {
				select {
				case <-ctx.Done():
					return
				case <-done:
					return
				default:
					req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
					if err != nil {
						if c.logger != nil {
							c.logger.Debug("Failed to create download request", "error", err)
						}
						return
					}

					resp, err := c.httpClient.Do(req)
					if err != nil {
						if c.logger != nil {
							c.logger.Debug("Download request failed", "error", err)
						}
						continue
					}

					// Read in chunks to track speed
					chunkSize := 64 * 1024 // 64KB chunks
					buffer := make([]byte, chunkSize)

				readLoop:
					for {
						select {
						case <-ctx.Done():
							if err := resp.Body.Close(); err != nil {
								if c.logger != nil {
									c.logger.Debug("Failed to close response body on context cancel", "error", err)
								}
							}
							return
						case <-done:
							if err := resp.Body.Close(); err != nil {
								if c.logger != nil {
									c.logger.Debug("Failed to close response body on done", "error", err)
								}
							}
							return
						default:
							n, err := resp.Body.Read(buffer)
							totalBytes.Add(int64(n))

							if err == io.EOF {
								break readLoop
							}
							if err != nil {
								break readLoop
							}
						}
					}
					if err := resp.Body.Close(); err != nil {
						if c.logger != nil {
							c.logger.Debug("Failed to close response body after read", "error", err)
						}
					}
				}
			}
		}(i)
	}

	// Sample aggregate throughput at 200ms intervals. The "max" we report is
	// the peak aggregate throughput observed during the test — readers like
	// fast.com show this rather than overall mean.
	measureTicker := time.NewTicker(200 * time.Millisecond)
	defer measureTicker.Stop()

	timeout := time.After(maxTime)
	doneOnce := sync.Once{}

	go func() {
		for {
			select {
			case <-timeout:
				doneOnce.Do(func() { close(done) })
				return
			case <-measureTicker.C:
				elapsed := time.Since(startTime).Seconds()
				if elapsed <= 0 {
					continue
				}

				bytes := totalBytes.Load()
				speedBps := float64(bytes*8) / elapsed // aggregate bits per second

				mu.Lock()
				if speedBps > maxSpeedBps {
					maxSpeedBps = speedBps
				}
				mu.Unlock()
			case <-ctx.Done():
				doneOnce.Do(func() { close(done) })
				return
			}
		}
	}()

	wg.Wait()
	doneOnce.Do(func() { close(done) })

	// Final measurement after all goroutines have finished — captures the
	// last bytes that arrived between the last tick and shutdown.
	if elapsed := time.Since(startTime).Seconds(); elapsed > 0 {
		finalBps := float64(totalBytes.Load()*8) / elapsed

		mu.Lock()
		if finalBps > maxSpeedBps {
			maxSpeedBps = finalBps
		}
		mu.Unlock()
	}

	mu.Lock()
	defer mu.Unlock()

	if maxSpeedBps == 0 {
		err := fmt.Errorf("no speed measurements recorded")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, err
	}

	// Convert bits per second to Mbps
	resultMbps := maxSpeedBps / 1e6
	span.SetAttributes(
		attribute.Float64("download.result_mbps", resultMbps),
		attribute.Float64("download.max_speed_bps", maxSpeedBps),
	)

	return resultMbps, nil
}
