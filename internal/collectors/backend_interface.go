package collectors

import (
	"context"
	"internet-perf-exporter/internal/config"
)

// TestResult represents the result of an internet performance test
type TestResult struct {
	Backend         string
	ServerID        string
	ServerLocation  string
	ServerName      string
	ServerCountry   string
	DownloadMbps    float64
	UploadMbps      float64
	LatencyMs       float64
	JitterMs        float64
	PacketLossPct   float64
	TestDurationSec float64
	Success         bool
	Error           error
}

// Backend defines the interface for internet performance test backends
type Backend interface {
	// Name returns the backend name (e.g., "speedtest", "fast")
	Name() string

	// RunTest executes a performance test and returns the result
	RunTest(ctx context.Context, cfg config.BackendConfig) (*TestResult, error)

	// IsEnabled returns whether this backend is enabled
	IsEnabled() bool
}

