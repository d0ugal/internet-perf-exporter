package collectors

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"internet-perf-exporter/internal/config"
	"internet-perf-exporter/internal/metrics"

	"github.com/d0ugal/promexporter/app"
	"github.com/d0ugal/promexporter/tracing"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/showwin/speedtest-go/speedtest"
	"go.opentelemetry.io/otel/attribute"
)

// SpeedtestCollector manages speedtest.net tests
type SpeedtestCollector struct {
	config  *config.Config
	metrics *metrics.InternetRegistry
	app     *app.App
	backend Backend
}

// NewSpeedtestCollector creates a new speedtest collector
func NewSpeedtestCollector(cfg *config.Config, registry *metrics.InternetRegistry, app *app.App) *SpeedtestCollector {
	backend := &SpeedtestBackend{}
	return &SpeedtestCollector{
		config:  cfg,
		metrics: registry,
		app:     app,
		backend: backend,
	}
}

// Stop stops the collector
func (sc *SpeedtestCollector) Stop() {
	// No cleanup needed
}

// Start starts the collector
func (sc *SpeedtestCollector) Start(ctx context.Context) {
	backendCfg, exists := sc.config.Backends["speedtest"]
	if !exists || !backendCfg.Enabled {
		slog.Info("Speedtest backend not enabled, skipping collector")
		return
	}

	go sc.run(ctx, backendCfg)
}

func (sc *SpeedtestCollector) run(ctx context.Context, backendCfg config.BackendConfig) {
	interval := backendCfg.Interval.Duration
	if interval == 0 {
		interval = time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial collection
	sc.collect(ctx, backendCfg)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Speedtest collector stopped")
			return
		case <-ticker.C:
			sc.collect(ctx, backendCfg)
		}
	}
}

func (sc *SpeedtestCollector) collect(ctx context.Context, backendCfg config.BackendConfig) {
	// Try to acquire the test lock to ensure only one test runs at a time
	coordinator := GetCoordinator()

	// Try to acquire lock with a timeout to avoid blocking indefinitely
	// If another test is running, skip this collection cycle
	if !coordinator.TryLock() {
		slog.Warn("Another test is currently running, skipping speedtest collection", "backend", "speedtest")
		return
	}

	// Ensure we unlock even if there's an error
	defer coordinator.Unlock()

	startTime := time.Now()
	interval := int(backendCfg.Interval.Seconds())

	// Create span for collection
	tracer := sc.app.GetTracer()
	var collectorSpan *tracing.CollectorSpan

	if tracer != nil && tracer.IsEnabled() {
		collectorSpan = tracer.NewCollectorSpan(ctx, "speedtest-collector", "collect-speedtest")
		ctx = collectorSpan.Context()
		defer collectorSpan.End()
	}

	slog.Info("Starting speedtest collection")

	result, err := sc.backend.RunTest(ctx, backendCfg)

	duration := time.Since(startTime).Seconds()

	if err != nil {
		slog.Error("Speedtest collection failed", "error", err)
		sc.metrics.CollectionFailed.With(prometheus.Labels{
			"backend":          "speedtest",
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

	sc.metrics.DownloadSpeedMbps.With(labels).Set(result.DownloadMbps)
	sc.metrics.UploadSpeedMbps.With(labels).Set(result.UploadMbps)

	// Only set latency if we have a valid value (skip 0 to avoid false outage alerts)
	if result.LatencyMs > 0 {
		sc.metrics.LatencyMs.With(labels).Observe(result.LatencyMs)
	}
	// Always set jitter and packet loss metrics when we have a test result
	// This includes when the value is 0 (actual measurement), not just default values
	sc.metrics.JitterMs.With(labels).Set(result.JitterMs)
	sc.metrics.PacketLossPct.With(labels).Set(result.PacketLossPct)

	sc.metrics.TestDurationSeconds.With(labels).Observe(result.TestDurationSec)

	if result.Success {
		sc.metrics.TestSuccess.With(labels).Set(1)
	} else {
		sc.metrics.TestSuccess.With(labels).Set(0)
		sc.metrics.TestFailedTotal.With(prometheus.Labels{
			"backend": "speedtest",
			"reason":  "test_failed",
		}).Inc()
	}

	// Server info
	infoLabels := prometheus.Labels{
		"backend":         result.Backend,
		"server_id":       result.ServerID,
		"server_location": result.ServerLocation,
		"server_name":     result.ServerName,
		"server_country":  result.ServerCountry,
	}
	sc.metrics.ServerInfo.With(infoLabels).Set(1)

	// Collection metrics
	collectionLabels := prometheus.Labels{
		"backend":          "speedtest",
		"interval_seconds": fmt.Sprintf("%d", interval),
	}
	sc.metrics.CollectionDuration.With(collectionLabels).Set(duration)
	sc.metrics.CollectionSuccess.With(collectionLabels).Inc()
	sc.metrics.CollectionTimestampGauge.With(collectionLabels).Set(float64(time.Now().Unix()))
	sc.metrics.CollectionIntervalGauge.With(prometheus.Labels{
		"backend": "speedtest",
	}).Set(float64(interval))

	slog.Info("Speedtest collection completed",
		"download_mbps", result.DownloadMbps,
		"upload_mbps", result.UploadMbps,
		"latency_ms", result.LatencyMs,
		"jitter_ms", result.JitterMs,
		"packet_loss_pct", result.PacketLossPct,
		"duration_seconds", duration)

	if collectorSpan != nil {
		collectorSpan.SetAttributes(
			attribute.Float64("download.mbps", result.DownloadMbps),
			attribute.Float64("upload.mbps", result.UploadMbps),
			attribute.Float64("latency.ms", result.LatencyMs),
			attribute.Float64("jitter.ms", result.JitterMs),
			attribute.Float64("packet_loss.pct", result.PacketLossPct),
			attribute.Float64("duration.seconds", duration),
		)
		collectorSpan.AddEvent("collection_completed")
	}
}

// SpeedtestBackend implements the Backend interface for speedtest.net
type SpeedtestBackend struct{}

func (sb *SpeedtestBackend) Name() string {
	return "speedtest"
}

func (sb *SpeedtestBackend) IsEnabled() bool {
	return true
}

func (sb *SpeedtestBackend) RunTest(ctx context.Context, cfg config.BackendConfig) (*TestResult, error) {
	// Create a context with timeout if configured
	testCtx := ctx
	if cfg.Timeout.Duration > 0 {
		var cancel context.CancelFunc
		testCtx, cancel = context.WithTimeout(ctx, cfg.Timeout.Duration)
		defer cancel()
	}

	// Fetch server list
	serverList, err := speedtest.FetchServerListContext(testCtx)
	if err != nil {
		return &TestResult{
			Backend:        "speedtest",
			ServerID:       "0",
			ServerLocation: "unknown",
			ServerName:     "unknown",
			ServerCountry:  "unknown",
			Success:        false,
			Error:          fmt.Errorf("failed to fetch server list: %w", err),
		}, err
	}

	if len(serverList) == 0 {
		return &TestResult{
			Backend:        "speedtest",
			ServerID:       "0",
			ServerLocation: "unknown",
			ServerName:     "unknown",
			ServerCountry:  "unknown",
			Success:        false,
			Error:          fmt.Errorf("no servers available"),
		}, fmt.Errorf("no servers available")
	}

	var targetServer *speedtest.Server

	// Select server based on config
	if cfg.Speedtest != nil {
		if cfg.Speedtest.ServerID > 0 {
			// Find server by ID
			for _, server := range serverList {
				if server.ID == fmt.Sprintf("%d", cfg.Speedtest.ServerID) {
					targetServer = server
					break
				}
			}
		} else if cfg.Speedtest.ServerName != "" {
			// Find server by name
			for _, server := range serverList {
				if server.Name == cfg.Speedtest.ServerName {
					targetServer = server
					break
				}
			}
		} else if cfg.Speedtest.ServerCountry != "" {
			// Find server by country
			for _, server := range serverList {
				if server.Country == cfg.Speedtest.ServerCountry {
					targetServer = server
					break
				}
			}
		}
	}

	// If no specific server selected, use closest (first in list)
	if targetServer == nil {
		targetServer = serverList[0]
	}

	slog.Debug("Using speedtest server",
		"server_id", targetServer.ID,
		"server_name", targetServer.Name,
		"server_country", targetServer.Country,
		"distance", targetServer.Distance)

	startTime := time.Now()

	// Test latency (ping) with context
	var latency time.Duration
	err = targetServer.PingTestContext(testCtx, func(result time.Duration) {
		latency = result
	})
	if err != nil {
		// Reset DataManager even on error to prevent memory accumulation
		if targetServer.Context != nil && targetServer.Context.Manager != nil {
			targetServer.Context.Manager.Reset()
		}
		return &TestResult{
			Backend:         "speedtest",
			ServerID:        targetServer.ID,
			ServerLocation:  targetServer.Name,
			ServerName:      targetServer.Name,
			ServerCountry:   targetServer.Country,
			LatencyMs:       float64(latency.Milliseconds()),
			TestDurationSec: time.Since(startTime).Seconds(),
			Success:         false,
			Error:           fmt.Errorf("ping test failed: %w", err),
		}, err
	}

	latencyMs := float64(latency.Milliseconds())

	// Reset DataManager to clear accumulated chunks from ping test
	if targetServer.Context != nil && targetServer.Context.Manager != nil {
		targetServer.Context.Manager.Reset()
	}

	// Run download test with context
	err = targetServer.DownloadTestContext(testCtx)
	if err != nil {
		// Reset DataManager even on error to prevent memory accumulation
		if targetServer.Context != nil && targetServer.Context.Manager != nil {
			targetServer.Context.Manager.Reset()
		}
		return &TestResult{
			Backend:         "speedtest",
			ServerID:        targetServer.ID,
			ServerLocation:  targetServer.Name,
			ServerName:      targetServer.Name,
			ServerCountry:   targetServer.Country,
			LatencyMs:       latencyMs,
			TestDurationSec: time.Since(startTime).Seconds(),
			Success:         false,
			Error:           fmt.Errorf("download test failed: %w", err),
		}, err
	}

	// Use the ByteRate.Mbps() method for accurate conversion
	downloadMbps := targetServer.DLSpeed.Mbps()

	// Reset DataManager to clear accumulated chunks from download test
	if targetServer.Context != nil && targetServer.Context.Manager != nil {
		targetServer.Context.Manager.Reset()
	}

	// Run upload test with context
	err = targetServer.UploadTestContext(testCtx)
	if err != nil {
		// Reset DataManager even on error to prevent memory accumulation
		if targetServer.Context != nil && targetServer.Context.Manager != nil {
			targetServer.Context.Manager.Reset()
		}
		return &TestResult{
			Backend:         "speedtest",
			ServerID:        targetServer.ID,
			ServerLocation:  targetServer.Name,
			ServerName:      targetServer.Name,
			ServerCountry:   targetServer.Country,
			DownloadMbps:    downloadMbps,
			LatencyMs:       latencyMs,
			TestDurationSec: time.Since(startTime).Seconds(),
			Success:         false,
			Error:           fmt.Errorf("upload test failed: %w", err),
		}, err
	}

	// Use the ByteRate.Mbps() method for accurate conversion
	uploadMbps := targetServer.ULSpeed.Mbps()
	duration := time.Since(startTime).Seconds()

	// Extract jitter and packet loss from server
	// Always extract values (even if 0) since they represent actual measurements
	jitterMs := float64(targetServer.Jitter.Milliseconds())

	// Use the LossPercent() method to get packet loss percentage
	packetLossPct := targetServer.PacketLoss.LossPercent()

	// Reset DataManager to clear accumulated chunks from upload test
	// This prevents memory accumulation across multiple test runs
	if targetServer.Context != nil && targetServer.Context.Manager != nil {
		targetServer.Context.Manager.Reset()
	}

	return &TestResult{
		Backend:         "speedtest",
		ServerID:        targetServer.ID,
		ServerLocation:  targetServer.Name,
		ServerName:      targetServer.Name,
		ServerCountry:   targetServer.Country,
		DownloadMbps:    downloadMbps,
		UploadMbps:      uploadMbps,
		LatencyMs:       latencyMs,
		JitterMs:        jitterMs,
		PacketLossPct:   packetLossPct,
		TestDurationSec: duration,
		Success:         true,
	}, nil
}
