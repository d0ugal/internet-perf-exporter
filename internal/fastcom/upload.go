package fastcom

import (
	"bytes"
	"context"
	"crypto/rand"
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

// runUploadTest performs upload speed test
func (c *Client) runUploadTest(ctx context.Context, maxTime time.Duration) (float64, error) {
	ctx, span := otel.Tracer("fastcom").Start(ctx, "fastcom.uploadTest")
	defer span.End()

	span.SetAttributes(
		attribute.String("upload.max_time", maxTime.String()),
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

	// totalBytes is shared across all upload goroutines so we compute
	// AGGREGATE throughput. The previous version computed
	//     speedBps := float64(payloadSize*8) / uploadDuration
	// per-POST and reported max(per-POST speed) — i.e. the throughput of
	// a single upload, undercounting real bandwidth ~Nx.
	var totalBytes atomic.Int64

	done := make(chan struct{})

	// Single global start so every measurement uses the same elapsed.
	startTime := time.Now()

	// Payload size for upload (10MB initial)
	payloadSize := 10 * 1024 * 1024

	// Start multiple concurrent uploads
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
					// Generate random payload
					payload := make([]byte, payloadSize)
					if _, err := rand.Read(payload); err != nil {
						if c.logger != nil {
							c.logger.Debug("Failed to generate random payload", "error", err)
						}
						continue
					}

					req, err := http.NewRequestWithContext(ctx, "POST", testURL, io.NopCloser(bytes.NewReader(payload)))
					if err != nil {
						if c.logger != nil {
							c.logger.Debug("Failed to create upload request", "error", err)
						}
						continue
					}

					req.ContentLength = int64(payloadSize)
					req.Header.Set("Content-Type", "application/octet-stream")

					resp, err := c.httpClient.Do(req)
					if err != nil {
						if c.logger != nil {
							c.logger.Debug("Upload request failed", "error", err)
						}
						continue
					}
					if err := resp.Body.Close(); err != nil {
						if c.logger != nil {
							c.logger.Debug("Failed to close response body after upload", "error", err)
						}
					}

					// We've sent payloadSize bytes by the time the server
					// has accepted the request. Add to the shared counter
					// for the next measurement tick to consume.
					totalBytes.Add(int64(payloadSize))
				}
			}
		}(i)
	}

	// Sample aggregate throughput at 200ms intervals — the "max" we report
	// is the peak aggregate throughput observed during the test.
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

	// Final measurement after all goroutines have finished.
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
		attribute.Float64("upload.result_mbps", resultMbps),
		attribute.Float64("upload.max_speed_bps", maxSpeedBps),
	)

	return resultMbps, nil
}

