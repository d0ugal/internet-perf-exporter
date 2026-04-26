package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"

	"internet-perf-exporter/internal/collectors"
	"internet-perf-exporter/internal/config"
	"internet-perf-exporter/internal/metrics"
	"internet-perf-exporter/internal/version"
	"github.com/d0ugal/promexporter/app"
	"github.com/d0ugal/promexporter/logging"
	promexporter_metrics "github.com/d0ugal/promexporter/metrics"
)

func main() {
	// Parse command line flags
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information")

	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.Parse()

	// Show version if requested
	if showVersion {
		slog.Info("internet-perf-exporter version", "version", version.Version, "commit", version.Commit, "build_date", version.BuildDate)
		os.Exit(0)
	}

	if configPath == "" {
		if envConfig := os.Getenv("CONFIG_PATH"); envConfig != "" {
			configPath = envConfig
		} else {
			configPath = "config.yaml"
		}
	}

	if os.Getenv("INTERNET_PERF_EXPORTER_CONFIG_FROM_ENV") == "true" {
		fmt.Fprintln(os.Stderr, "Warning: INTERNET_PERF_EXPORTER_CONFIG_FROM_ENV is deprecated and has no effect. Env vars are always applied on top of yaml config.")
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err, "path", configPath)
		os.Exit(1)
	}

	// Configure logging
	logging.Configure(&logging.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})

	// Initialize metrics registry using promexporter
	metricsRegistry := promexporter_metrics.NewRegistry("internet_perf_exporter_info")

	// Add custom metrics to the registry
	internetRegistry := metrics.NewInternetRegistry(metricsRegistry)

	// Create and build application using promexporter
	application := app.New("Internet Performance Exporter").
		WithConfig(&cfg.BaseConfig).
		WithMetrics(metricsRegistry).
		WithVersionInfo(version.Version, version.Commit, version.BuildDate).
		Build()

	// Create collectors with app reference for tracing
	speedtestCollector := collectors.NewSpeedtestCollector(cfg, internetRegistry, application)
	fastCollector := collectors.NewFastCollector(cfg, internetRegistry, application)
	application.WithCollector(speedtestCollector)
	application.WithCollector(fastCollector)

	// Run the application
	if err := application.Run(); err != nil {
		log.Fatalf("Failed to run application: %v", err)
	}
}

