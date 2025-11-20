package fastcom

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
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

	done := make(chan struct{})
	// Limit speeds slice to prevent unbounded memory growth
	// We only need recent measurements for averaging, so keep a reasonable buffer
	const maxSpeedEntries = 100
	speeds := make([]float64, 0, maxSpeedEntries)

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

			startTime := time.Now()
			var totalBytes int64
			lastMeasurementTime := time.Now()
			const measurementInterval = 200 * time.Millisecond // Throttle measurements to reduce memory pressure

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
							totalBytes += int64(n)

							// Throttle measurements to reduce memory pressure and lock contention
							// Only record measurements periodically, not on every read
							now := time.Now()
							if now.Sub(lastMeasurementTime) >= measurementInterval {
								elapsed := time.Since(startTime).Seconds()
								if elapsed > 0 {
									speedBps := float64(totalBytes*8) / elapsed // bits per second

									mu.Lock()
									if speedBps > maxSpeedBps {
										maxSpeedBps = speedBps
									}
									// Store periodic measurements with bounded growth
									// Keep only recent measurements to prevent memory leaks
									if len(speeds) >= maxSpeedEntries {
										// Remove oldest entry (FIFO)
										copy(speeds, speeds[1:])
										speeds = speeds[:len(speeds)-1]
									}
									speeds = append(speeds, speedBps)
									mu.Unlock()
								}
								lastMeasurementTime = now
							}

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

	// Collect measurements over time
	measureTicker := time.NewTicker(1 * time.Second)
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
				mu.Lock()
				if len(speeds) > 0 {
					// Use recent measurements (last window)
					windowSize := 3
					if len(speeds) < windowSize {
						windowSize = len(speeds)
					}
					recent := speeds[len(speeds)-windowSize:]
					var sum float64
					for _, s := range recent {
						sum += s
					}
					avg := sum / float64(len(recent))
					if avg > maxSpeedBps {
						maxSpeedBps = avg
					}
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
