package collectors

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"internet-perf-exporter/internal/config"
	"internet-perf-exporter/internal/fastcom"
	"internet-perf-exporter/internal/metrics"

	"github.com/d0ugal/promexporter/app"
	"github.com/d0ugal/promexporter/tracing"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
)

// FastCollector manages fast.com tests
type FastCollector struct {
	config  *config.Config
	metrics *metrics.InternetRegistry
	app     *app.App
	backend Backend
}

// NewFastCollector creates a new fast.com collector
func NewFastCollector(cfg *config.Config, registry *metrics.InternetRegistry, app *app.App) *FastCollector {
	backend := &FastBackend{}
	return &FastCollector{
		config:  cfg,
		metrics: registry,
		app:     app,
		backend: backend,
	}
}

// Stop stops the collector
func (fc *FastCollector) Stop() {
	// No cleanup needed
}

// Start starts the collector
func (fc *FastCollector) Start(ctx context.Context) {
	backendCfg, exists := fc.config.Backends["fast"]
	if !exists || !backendCfg.Enabled {
		slog.Info("Fast.com backend not enabled, skipping collector")
		return
	}

	go fc.run(ctx, backendCfg)
}

func (fc *FastCollector) run(ctx context.Context, backendCfg config.BackendConfig) {
	interval := backendCfg.Interval.Duration
	if interval == 0 {
		interval = time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial collection
	fc.collect(ctx, backendCfg)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Fast.com collector stopped")
			return
		case <-ticker.C:
			fc.collect(ctx, backendCfg)
		}
	}
}

func (fc *FastCollector) collect(ctx context.Context, backendCfg config.BackendConfig) {
	// Try to acquire the test lock to ensure only one test runs at a time
	coordinator := GetCoordinator()

	// Try to acquire lock with a timeout to avoid blocking indefinitely
	// If another test is running, skip this collection cycle
	if !coordinator.TryLock() {
		slog.Warn("Another test is currently running, skipping fast.com collection", "backend", "fast")
		return
	}

	// Ensure we unlock even if there's an error
	defer coordinator.Unlock()

	startTime := time.Now()
	interval := int(backendCfg.Interval.Seconds())

	// Create span for collection
	tracer := fc.app.GetTracer()
	var collectorSpan *tracing.CollectorSpan

	if tracer != nil && tracer.IsEnabled() {
		collectorSpan = tracer.NewCollectorSpan(ctx, "fast-collector", "collect-fast")
		ctx = collectorSpan.Context()
		defer collectorSpan.End()
	}

	slog.Info("Starting fast.com collection")

	result, err := fc.backend.RunTest(ctx, backendCfg)

	duration := time.Since(startTime).Seconds()

	if err != nil {
		slog.Error("Fast.com collection failed", "error", err)
		fc.metrics.CollectionFailed.With(prometheus.Labels{
			"backend":          "fast",
			"interval_seconds": fmt.Sprintf("%d", interval),
		}).Inc()
		if collectorSpan != nil {
			collectorSpan.RecordError(err)
		}
		return
	}

	// Record metrics
	labels := prometheus.Labels{
		"backend":         result.Backend,
		"server_id":       result.ServerID,
		"server_location": result.ServerLocation,
	}

	fc.metrics.DownloadSpeedMbps.With(labels).Set(result.DownloadMbps)

	// Set upload metric if we have a valid value (now supported by our custom client)
	if result.UploadMbps > 0 {
		fc.metrics.UploadSpeedMbps.With(labels).Set(result.UploadMbps)
	}

	// Set latency if we have a valid value (now supported by our custom client)
	if result.LatencyMs > 0 {
		fc.metrics.LatencyMs.With(labels).Observe(result.LatencyMs)
	}
	if result.JitterMs > 0 {
		fc.metrics.JitterMs.With(labels).Set(result.JitterMs)
	}
	if result.PacketLossPct > 0 {
		fc.metrics.PacketLossPct.With(labels).Set(result.PacketLossPct)
	}

	fc.metrics.TestDurationSeconds.With(labels).Observe(result.TestDurationSec)

	if result.Success {
		fc.metrics.TestSuccess.With(labels).Set(1)
	} else {
		fc.metrics.TestSuccess.With(labels).Set(0)
		fc.metrics.TestFailedTotal.With(prometheus.Labels{
			"backend": "fast",
			"reason":  "test_failed",
		}).Inc()
	}

	// Server info (fast.com doesn't have server selection, but we still record it)
	infoLabels := prometheus.Labels{
		"backend":         result.Backend,
		"server_id":       result.ServerID,
		"server_location": result.ServerLocation,
		"server_name":     result.ServerName,
		"server_country":  result.ServerCountry,
	}
	fc.metrics.ServerInfo.With(infoLabels).Set(1)

	// Collection metrics
	collectionLabels := prometheus.Labels{
		"backend":          "fast",
		"interval_seconds": fmt.Sprintf("%d", interval),
	}
	fc.metrics.CollectionDuration.With(collectionLabels).Set(duration)
	fc.metrics.CollectionSuccess.With(collectionLabels).Inc()
	fc.metrics.CollectionTimestampGauge.With(collectionLabels).Set(float64(time.Now().Unix()))
	fc.metrics.CollectionIntervalGauge.With(prometheus.Labels{
		"backend": "fast",
	}).Set(float64(interval))

	slog.Info("Fast.com collection completed",
		"download_mbps", result.DownloadMbps,
		"upload_mbps", result.UploadMbps,
		"latency_ms", result.LatencyMs,
		"duration_seconds", duration)

	if collectorSpan != nil {
		collectorSpan.SetAttributes(
			attribute.Float64("download.mbps", result.DownloadMbps),
			attribute.Float64("upload.mbps", result.UploadMbps),
			attribute.Float64("latency.ms", result.LatencyMs),
			attribute.Float64("duration.seconds", duration),
		)
		collectorSpan.AddEvent("collection_completed")
	}
}

// FastBackend implements the Backend interface for fast.com
type FastBackend struct{}

func (fb *FastBackend) Name() string {
	return "fast"
}

func (fb *FastBackend) IsEnabled() bool {
	return true
}

func (fb *FastBackend) RunTest(ctx context.Context, cfg config.BackendConfig) (*TestResult, error) {
	startTime := time.Now()

	// Determine max time for test
	maxTime := cfg.Timeout.Duration
	if maxTime == 0 {
		maxTime = 15 * time.Second // Default to 15 seconds
	}

	// Create Fast.com client and run test
	client := fastcom.NewClient()
	fastResult, err := client.RunTest(ctx, maxTime)

	duration := time.Since(startTime).Seconds()

	if err != nil {
		return &TestResult{
			Backend:         "fast",
			ServerID:        "fast-com",
			ServerLocation:  "auto",
			ServerName:      "Fast.com",
			ServerCountry:   "US",
			TestDurationSec: duration,
			Success:         false,
			Error:           fmt.Errorf("failed to run fast.com test: %w", err),
		}, err
	}

	return &TestResult{
		Backend:         "fast",
		ServerID:        "fast-com",
		ServerLocation:  "auto",
		ServerName:      "Fast.com",
		ServerCountry:   "US",
		DownloadMbps:    fastResult.DownloadMbps,
		UploadMbps:      fastResult.UploadMbps,
		LatencyMs:       fastResult.LatencyMs,
		JitterMs:        0, // Fast.com doesn't provide jitter
		PacketLossPct:   0, // Fast.com doesn't provide packet loss
		TestDurationSec: duration,
		Success:         true,
	}, nil
}
