// Package fastcom provides a Go client for testing internet speed using Fast.com (Netflix's speed test service).
// It supports download, upload, and latency measurements by communicating directly with Fast.com's API.
//
// Example usage:
//
//	client := fastcom.NewClient()
//	result, err := client.RunTest(ctx, 15*time.Second)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("Download: %.2f Mbps, Upload: %.2f Mbps, Latency: %.2f ms\n",
//		result.DownloadMbps, result.UploadMbps, result.LatencyMs)
package fastcom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// Client is a Fast.com speed test client
type Client struct {
	httpClient *http.Client
	logger     *slog.Logger
	token      string
	urls       []string
}

// Result contains the results of a Fast.com speed test
type Result struct {
	// DownloadMbps is the measured download speed in megabits per second
	DownloadMbps float64
	
	// UploadMbps is the measured upload speed in megabits per second
	UploadMbps float64
	
	// LatencyMs is the unloaded ping latency in milliseconds
	LatencyMs float64
	
	// LoadedLatencyMs is the latency during download test (currently not measured)
	LoadedLatencyMs float64
}

// Config allows customization of the client behavior
type Config struct {
	// HTTPClient is the HTTP client to use for requests. If nil, a default client is created.
	HTTPClient *http.Client
	
	// Logger is the logger to use for debug messages. If nil, no logging is performed.
	Logger *slog.Logger
}

// NewClient creates a new Fast.com client with default settings
func NewClient() *Client {
	return NewClientWithConfig(Config{})
}

// NewClientWithConfig creates a new Fast.com client with custom configuration
func NewClientWithConfig(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		// Create an instrumented HTTP client with otelhttp
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport,
				otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
					return r.Method + " " + r.URL.Host
				}),
			),
		}
	}

	return &Client{
		httpClient: httpClient,
		logger:     cfg.Logger,
	}
}

// getToken fetches the token from fast.com's JavaScript file
func (c *Client) getToken(ctx context.Context) (string, error) {
	ctx, span := otel.Tracer("fastcom").Start(ctx, "fastcom.getToken")
	defer span.End()

	logger := c.logger
	
	// Fetch fast.com homepage
	req, err := http.NewRequestWithContext(ctx, "GET", "https://fast.com/", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("failed to fetch fast.com homepage: %w", err)
	}
	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
	defer func() {
		if err := resp.Body.Close(); err != nil {
			if logger != nil {
				logger.Debug("Failed to close response body", "error", err)
			}
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Extract JavaScript filename from HTML
	jsPattern := regexp.MustCompile(`script src="([^"]+app-[^"]+\.js)"`)
	matches := jsPattern.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return "", fmt.Errorf("could not find JavaScript file in HTML")
	}

	jsURL := "https://fast.com" + matches[1]
	if !strings.HasPrefix(jsURL, "https://") {
		jsURL = "https://fast.com" + matches[1]
	}

	if logger != nil {
		logger.Debug("Found JavaScript URL", "url", jsURL)
	}
	span.SetAttributes(attribute.String("js.url", jsURL))

	// Fetch JavaScript file
	req, err = http.NewRequestWithContext(ctx, "GET", jsURL, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("failed to create request for JS file: %w", err)
	}

	resp, err = c.httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("failed to fetch JavaScript file: %w", err)
	}
	span.SetAttributes(attribute.Int("js.http.status_code", resp.StatusCode))
	defer func() {
		if err := resp.Body.Close(); err != nil {
			if c.logger != nil {
				c.logger.Debug("Failed to close response body", "error", err)
			}
		}
	}()

	jsBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read JavaScript: %w", err)
	}

	// Extract token from JavaScript
	tokenPattern := regexp.MustCompile(`token:"([^"]+)"`)
	tokenMatches := tokenPattern.FindStringSubmatch(string(jsBody))
	if len(tokenMatches) < 2 {
		return "", fmt.Errorf("could not find token in JavaScript")
	}

	token := tokenMatches[1]
	if c.logger != nil {
		c.logger.Debug("Extracted token from JavaScript")
	}
	span.SetAttributes(attribute.Bool("token.extracted", true))

	return token, nil
}

// getTestURLs fetches test URLs from Fast.com API
func (c *Client) getTestURLs(ctx context.Context, token string) ([]string, error) {
	ctx, span := otel.Tracer("fastcom").Start(ctx, "fastcom.getTestURLs")
	defer span.End()

	apiURL := fmt.Sprintf("https://api.fast.com/netflix/speedtest?https=true&token=%s&urlCount=3", token)
	span.SetAttributes(attribute.String("api.url", "https://api.fast.com/netflix/speedtest"))
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to fetch test URLs: %w", err)
	}
	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
	defer func() {
		if err := resp.Body.Close(); err != nil {
			if c.logger != nil {
				c.logger.Debug("Failed to close response body", "error", err)
			}
		}
	}()

	var result []struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	urls := make([]string, len(result))
	for i, item := range result {
		urls[i] = item.URL
	}

	if c.logger != nil {
		c.logger.Debug("Retrieved test URLs", "count", len(urls))
	}
	span.SetAttributes(attribute.Int("urls.count", len(urls)))

	return urls, nil
}

// extractHostname extracts the hostname from a URL
func extractHostname(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	return parsedURL.Hostname(), nil
}

// pingHost performs a TCP ping to the host and returns latency in milliseconds
// We use TCP instead of ICMP since many systems don't allow raw ICMP sockets
func pingHost(host string, count int, timeout time.Duration) (float64, error) {
	var totalRTT time.Duration
	successCount := 0

	// Try HTTPS first (port 443), then HTTP (port 80)
	ports := []string{"443", "80"}
	
	for _, port := range ports {
		for i := 0; i < count; i++ {
			start := time.Now()
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
			if err != nil {
				continue
			}
			if err := conn.Close(); err != nil {
				// Connection close errors during ping are typically not critical
				continue
			}
			rtt := time.Since(start)
			totalRTT += rtt
			successCount++
			
			// Small delay between pings
			time.Sleep(100 * time.Millisecond)
		}
		
		if successCount > 0 {
			break
		}
	}

	if successCount == 0 {
		return 0, fmt.Errorf("no successful pings")
	}

	avgRTT := totalRTT / time.Duration(successCount)
	return float64(avgRTT.Nanoseconds()) / 1e6, nil
}


// RunTest runs a complete Fast.com speed test including download, upload, and ping.
// The maxTime parameter controls how long each test phase should run.
func (c *Client) RunTest(ctx context.Context, maxTime time.Duration) (*Result, error) {
	ctx, span := otel.Tracer("fastcom").Start(ctx, "fastcom.RunTest")
	defer span.End()

	span.SetAttributes(
		attribute.String("test.max_time", maxTime.String()),
	)

	// Get token
	token, err := c.getToken(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get token")
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	c.token = token
	span.SetAttributes(attribute.Bool("token.obtained", true))

	// Get test URLs
	urls, err := c.getTestURLs(ctx, token)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get test URLs")
		return nil, fmt.Errorf("failed to get test URLs: %w", err)
	}
	c.urls = urls
	span.SetAttributes(attribute.Int("urls.count", len(urls)))

	if len(urls) == 0 {
		err := fmt.Errorf("no URLs returned from API")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Extract hostname for ping test
	hostname, err := extractHostname(urls[0])
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to extract hostname: %w", err)
	}
	span.SetAttributes(attribute.String("test.hostname", hostname))

	result := &Result{}

	// Run ping test (unloaded)
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	
	pingCtx, pingSpan := otel.Tracer("fastcom").Start(pingCtx, "fastcom.pingTest")
	pingSpan.SetAttributes(attribute.String("ping.hostname", hostname))
	
	// Run ping in goroutine with context
	pingDone := make(chan struct {
		latency float64
		err     error
	}, 1)
	go func() {
		latency, err := pingHost(hostname, 5, 2*time.Second)
		pingDone <- struct {
			latency float64
			err     error
		}{latency, err}
	}()

	select {
	case <-pingCtx.Done():
		if c.logger != nil {
			c.logger.Debug("Ping test timed out")
		}
		pingSpan.SetAttributes(attribute.Bool("ping.timeout", true))
	case pingResult := <-pingDone:
		if pingResult.err == nil {
			result.LatencyMs = pingResult.latency
			pingSpan.SetAttributes(attribute.Float64("ping.latency_ms", pingResult.latency))
		} else {
			pingSpan.RecordError(pingResult.err)
			if c.logger != nil {
				c.logger.Debug("Ping test failed, skipping", "error", pingResult.err)
			}
		}
	}
	pingSpan.End()

	// Run download test
	downloadCtx, downloadCancel := context.WithTimeout(ctx, maxTime)
	defer downloadCancel()
	
	downloadMbps, err := c.runDownloadTest(downloadCtx, maxTime)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "download test failed")
		return nil, fmt.Errorf("download test failed: %w", err)
	}
	result.DownloadMbps = downloadMbps
	span.SetAttributes(attribute.Float64("test.download_mbps", downloadMbps))

	// Run upload test
	uploadCtx, uploadCancel := context.WithTimeout(ctx, maxTime)
	defer uploadCancel()
	
	uploadMbps, err := c.runUploadTest(uploadCtx, maxTime)
	if err != nil {
		// Upload might fail, but don't fail the entire test
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("test.upload_failed", true))
		if c.logger != nil {
			c.logger.Debug("Upload test failed", "error", err)
		}
		result.UploadMbps = 0
	} else {
		result.UploadMbps = uploadMbps
		span.SetAttributes(attribute.Float64("test.upload_mbps", uploadMbps))
	}

	span.SetAttributes(
		attribute.Float64("result.download_mbps", result.DownloadMbps),
		attribute.Float64("result.upload_mbps", result.UploadMbps),
		attribute.Float64("result.latency_ms", result.LatencyMs),
	)

	return result, nil
}

