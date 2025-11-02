package fastcom

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// runUploadTest performs upload speed test
func (c *Client) runUploadTest(ctx context.Context, maxTime time.Duration) (float64, error) {
	if len(c.urls) == 0 {
		return 0, fmt.Errorf("no test URLs available")
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var maxSpeedBps float64
	
	done := make(chan struct{})
	speeds := make([]float64, 0)

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
			
			startTime := time.Now()

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

					uploadStart := time.Now()
					resp, err := c.httpClient.Do(req)
					if err != nil {
						if c.logger != nil {
							c.logger.Debug("Upload request failed", "error", err)
						}
						continue
					}
					uploadDuration := time.Since(uploadStart)
					if err := resp.Body.Close(); err != nil {
						if c.logger != nil {
							c.logger.Debug("Failed to close response body after upload", "error", err)
						}
					}

					if uploadDuration.Seconds() > 0 {
						// Calculate speed: (payloadSize * 8 bits) / duration in seconds
						speedBps := float64(payloadSize*8) / uploadDuration.Seconds()
						
						mu.Lock()
						speeds = append(speeds, speedBps)
						if speedBps > maxSpeedBps {
							maxSpeedBps = speedBps
						}
						mu.Unlock()
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

