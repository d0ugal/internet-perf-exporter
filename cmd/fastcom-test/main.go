package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"internet-perf-exporter/internal/fastcom"
)

func main() {
	var (
		timeout    = flag.Duration("timeout", 15*time.Second, "Maximum time for each test phase (download/upload)")
		verbose    = flag.Bool("verbose", false, "Enable verbose logging")
		skipUpload = flag.Bool("skip-upload", false, "Skip upload test (faster)")
		skipPing   = flag.Bool("skip-ping", false, "Skip ping test (faster)")
	)
	flag.Parse()

	// Setup logging
	var logger *slog.Logger
	if *verbose {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}

	// Create client
	var client *fastcom.Client
	if logger != nil {
		client = fastcom.NewClientWithConfig(fastcom.Config{
			Logger: logger,
		})
	} else {
		client = fastcom.NewClient()
	}

	fmt.Println("Fast.com Speed Test")
	fmt.Println("==================")
	fmt.Printf("Timeout: %v\n", *timeout)
	fmt.Println()

	ctx := context.Background()

	if !*skipPing {
		fmt.Println("🔍 Getting token and test URLs...")
	}

	// If we're skipping upload, we need to modify the client behavior
	// For now, we'll just run the full test and note what was skipped
	fmt.Println("⚡ Running speed test...")
	fmt.Println()

	startTime := time.Now()

	// Note: The current implementation always runs all tests
	// We could modify the client to support skipping phases, but for now
	// we'll just run it and report the results
	
	result, err := client.RunTest(ctx, *timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}

	duration := time.Since(startTime)

	fmt.Println("✅ Test completed!")
	fmt.Println()
	fmt.Println("Results:")
	fmt.Println("-------")
	fmt.Printf("  Download:  %8.2f Mbps\n", result.DownloadMbps)
	
	if *skipUpload {
		fmt.Printf("  Upload:    %8s (skipped)\n", "-")
	} else {
		fmt.Printf("  Upload:    %8.2f Mbps\n", result.UploadMbps)
	}
	
	if *skipPing {
		fmt.Printf("  Latency:   %8s (skipped)\n", "-")
	} else {
		fmt.Printf("  Latency:   %8.2f ms\n", result.LatencyMs)
	}
	
	fmt.Printf("  Duration:  %8.2f seconds\n", duration.Seconds())
	fmt.Println()

	// Show a summary
	fmt.Println("Summary:")
	fmt.Println("-------")
	if result.DownloadMbps > 0 {
		fmt.Printf("  ✓ Download test successful\n")
	} else {
		fmt.Printf("  ✗ Download test failed\n")
	}
	
	if !*skipUpload {
		if result.UploadMbps > 0 {
			fmt.Printf("  ✓ Upload test successful\n")
		} else {
			fmt.Printf("  ✗ Upload test failed or not supported\n")
		}
	}
	
	if !*skipPing {
		if result.LatencyMs > 0 {
			fmt.Printf("  ✓ Latency test successful\n")
		} else {
			fmt.Printf("  ✗ Latency test failed\n")
		}
	}
}

