package metrics

import (
	promexporter_metrics "github.com/d0ugal/promexporter/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// InternetRegistry wraps the promexporter registry with internet performance metrics
type InternetRegistry struct {
	*promexporter_metrics.Registry

	// Speed metrics (documented)
	DownloadSpeedMbps *prometheus.GaugeVec
	UploadSpeedMbps   *prometheus.GaugeVec

	// Latency metrics (documented)
	LatencyMs    *prometheus.HistogramVec
	JitterMs     *prometheus.GaugeVec
	PacketLossPct *prometheus.GaugeVec

	// Test metrics (documented)
	TestDurationSeconds *prometheus.HistogramVec
	TestSuccess         *prometheus.GaugeVec
	TestFailedTotal     *prometheus.CounterVec

	// Server info (documented)
	ServerInfo *prometheus.GaugeVec

	// Collection metrics (documented)
	CollectionDuration *prometheus.GaugeVec
	CollectionSuccess  *prometheus.CounterVec
	CollectionFailed   *prometheus.CounterVec

	// Additional operational metrics
	CollectionIntervalGauge  *prometheus.GaugeVec
	CollectionTimestampGauge *prometheus.GaugeVec
}

// NewInternetRegistry creates a new internet performance metrics registry
func NewInternetRegistry(baseRegistry *promexporter_metrics.Registry) *InternetRegistry {
	registry := &InternetRegistry{
		Registry: baseRegistry,

		// Speed metrics
		DownloadSpeedMbps: promauto.With(baseRegistry.GetRegistry()).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "internet_perf_exporter_download_speed_mbps",
				Help: "Download speed in Mbps",
			},
			[]string{"backend", "server_id", "server_location"},
		),
		UploadSpeedMbps: promauto.With(baseRegistry.GetRegistry()).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "internet_perf_exporter_upload_speed_mbps",
				Help: "Upload speed in Mbps",
			},
			[]string{"backend", "server_id", "server_location"},
		),

		// Latency metrics
		LatencyMs: promauto.With(baseRegistry.GetRegistry()).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "internet_perf_exporter_latency_ms",
				Help:    "Latency in milliseconds",
				Buckets: prometheus.ExponentialBuckets(1, 2, 12), // 1ms to ~2s
			},
			[]string{"backend", "server_id", "server_location"},
		),
		JitterMs: promauto.With(baseRegistry.GetRegistry()).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "internet_perf_exporter_jitter_ms",
				Help: "Jitter in milliseconds",
			},
			[]string{"backend", "server_id", "server_location"},
		),
		PacketLossPct: promauto.With(baseRegistry.GetRegistry()).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "internet_perf_exporter_packet_loss_percent",
				Help: "Packet loss percentage (0-100)",
			},
			[]string{"backend", "server_id", "server_location"},
		),

		// Test metrics
		TestDurationSeconds: promauto.With(baseRegistry.GetRegistry()).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "internet_perf_exporter_test_duration_seconds",
				Help:    "Duration of speed test in seconds",
				Buckets: prometheus.ExponentialBuckets(1, 2, 10), // 1s to ~8.5 minutes
			},
			[]string{"backend", "server_id", "server_location"},
		),
		TestSuccess: promauto.With(baseRegistry.GetRegistry()).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "internet_perf_exporter_test_success",
				Help: "Test success status (1=success, 0=failure)",
			},
			[]string{"backend", "server_id", "server_location"},
		),
		TestFailedTotal: promauto.With(baseRegistry.GetRegistry()).NewCounterVec(
			prometheus.CounterOpts{
				Name: "internet_perf_exporter_test_failed_total",
				Help: "Total number of failed tests",
			},
			[]string{"backend", "reason"},
		),

		// Server info
		ServerInfo: promauto.With(baseRegistry.GetRegistry()).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "internet_perf_exporter_server_info",
				Help: "Information about the test server",
			},
			[]string{"backend", "server_id", "server_location", "server_name", "server_country"},
		),

		// Collection metrics
		CollectionDuration: promauto.With(baseRegistry.GetRegistry()).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "internet_perf_exporter_collection_duration_seconds",
				Help: "Duration of collection in seconds",
			},
			[]string{"backend", "interval_seconds"},
		),
		CollectionSuccess: promauto.With(baseRegistry.GetRegistry()).NewCounterVec(
			prometheus.CounterOpts{
				Name: "internet_perf_exporter_collection_success_total",
				Help: "Total number of successful collections",
			},
			[]string{"backend", "interval_seconds"},
		),
		CollectionFailed: promauto.With(baseRegistry.GetRegistry()).NewCounterVec(
			prometheus.CounterOpts{
				Name: "internet_perf_exporter_collection_failed_total",
				Help: "Total number of failed collections",
			},
			[]string{"backend", "interval_seconds"},
		),

		// Additional operational metrics
		CollectionIntervalGauge: promauto.With(baseRegistry.GetRegistry()).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "internet_perf_exporter_collection_interval_seconds",
				Help: "Collection interval in seconds",
			},
			[]string{"backend"},
		),
		CollectionTimestampGauge: promauto.With(baseRegistry.GetRegistry()).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "internet_perf_exporter_collection_timestamp",
				Help: "Timestamp of collection",
			},
			[]string{"backend", "interval_seconds"},
		),
	}

	// Add metric metadata for UI
	registry.AddMetricInfo("internet_perf_exporter_info", "Information about the internet performance exporter", []string{"version", "commit", "build_date"})
	registry.AddMetricInfo("internet_perf_exporter_download_speed_mbps", "Download speed in Mbps", []string{"backend", "server_id", "server_location"})
	registry.AddMetricInfo("internet_perf_exporter_upload_speed_mbps", "Upload speed in Mbps", []string{"backend", "server_id", "server_location"})
	registry.AddMetricInfo("internet_perf_exporter_latency_ms", "Latency in milliseconds", []string{"backend", "server_id", "server_location"})
	registry.AddMetricInfo("internet_perf_exporter_jitter_ms", "Jitter in milliseconds", []string{"backend", "server_id", "server_location"})
	registry.AddMetricInfo("internet_perf_exporter_packet_loss_percent", "Packet loss percentage (0-100)", []string{"backend", "server_id", "server_location"})
	registry.AddMetricInfo("internet_perf_exporter_test_duration_seconds", "Duration of speed test in seconds", []string{"backend", "server_id", "server_location"})
	registry.AddMetricInfo("internet_perf_exporter_test_success", "Test success status (1=success, 0=failure)", []string{"backend", "server_id", "server_location"})
	registry.AddMetricInfo("internet_perf_exporter_test_failed_total", "Total number of failed tests", []string{"backend", "reason"})
	registry.AddMetricInfo("internet_perf_exporter_server_info", "Information about the test server", []string{"backend", "server_id", "server_location", "server_name", "server_country"})
	registry.AddMetricInfo("internet_perf_exporter_collection_duration_seconds", "Duration of collection in seconds", []string{"backend", "interval_seconds"})
	registry.AddMetricInfo("internet_perf_exporter_collection_success_total", "Total number of successful collections", []string{"backend", "interval_seconds"})
	registry.AddMetricInfo("internet_perf_exporter_collection_failed_total", "Total number of failed collections", []string{"backend", "interval_seconds"})

	return registry
}

