package fastcom

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// runDownloadTest performs download speed test
func (c *Client) runDownloadTest(ctx context.Context, maxTime time.Duration) (float64, error) {
	if len(c.urls) == 0 {
		return 0, fmt.Errorf("no test URLs available")
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var maxSpeedBps float64

	done := make(chan struct{})
	speeds := make([]float64, 0)

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

							elapsed := time.Since(startTime).Seconds()
							if elapsed > 0 {
								speedBps := float64(totalBytes*8) / elapsed // bits per second

								mu.Lock()
								if speedBps > maxSpeedBps {
									maxSpeedBps = speedBps
								}
								// Store periodic measurements
								speeds = append(speeds, speedBps)
								mu.Unlock()
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
		return 0, fmt.Errorf("no speed measurements recorded")
	}

	// Convert bits per second to Mbps
	return maxSpeedBps / 1e6, nil
}
