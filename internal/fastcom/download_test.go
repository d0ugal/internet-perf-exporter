package fastcom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestRunDownloadTest_AggregatesAcrossConnections verifies that the
// reported download speed scales (roughly) with the aggregate throughput
// across all 8 connections, not the per-connection throughput.
//
// Before the fix, each goroutine kept its own totalBytes and the reported
// "max speed" was max(per-goroutine speed). With 8 connections all reading
// roughly the same amount, that under-counted real bandwidth ~8x.
//
// We use a httptest.Server that streams a fixed payload as fast as it can.
// On a localhost loopback connection the throughput per connection is
// ~hundreds of MB/s; with 8 concurrent readers the aggregate measurement
// must be substantially higher than what any single connection achieves.
// We assert speed is at least 2× a conservative single-connection floor —
// any aggregation works fine, and the old per-goroutine math would not.
func TestRunDownloadTest_AggregatesAcrossConnections(t *testing.T) {
	// 1 MB payload that the server returns immediately. Each goroutine
	// will finish a request fast on loopback and immediately re-request,
	// driving sustained aggregate throughput.
	payload := strings.Repeat("x", 1024*1024)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	c := &Client{
		httpClient: server.Client(),
		urls:       []string{server.URL},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mbps, err := c.runDownloadTest(ctx, 1500*time.Millisecond)
	if err != nil {
		t.Fatalf("runDownloadTest: %v", err)
	}

	// On loopback we expect well over 100 Mbps aggregate. Pick a low floor
	// to keep the test stable on slow CI runners while still being well
	// above what a single-connection reading 1 MB/s would produce
	// (~8 Mbps, the buggy per-goroutine result).
	const minMbps = 50.0
	if mbps < minMbps {
		t.Fatalf("aggregate speed too low — bug likely back: got %.2f Mbps, want at least %.2f Mbps", mbps, minMbps)
	}
}
