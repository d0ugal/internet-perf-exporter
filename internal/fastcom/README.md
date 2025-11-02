# Fast.com Go Client

A Go library for testing internet speed using Fast.com (Netflix's speed test service).

## Features

- ✅ Download speed testing
- ✅ Upload speed testing  
- ✅ Latency (ping) measurement
- ✅ Minimal dependencies (only standard library + log/slog)
- ✅ Context-aware for cancellation and timeouts
- ✅ Configurable HTTP client and logging

## Installation

This is currently part of the `internet-perf-exporter` project, but it can be easily extracted into its own module.

To extract this into a standalone library:

1. Create a new Go module: `go mod init github.com/yourusername/fastcom`
2. Copy the `internal/fastcom` directory contents
3. Update package documentation and examples

## Usage

### Basic Example

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/yourusername/fastcom"
)

func main() {
    ctx := context.Background()
    
    // Create a client
    client := fastcom.NewClient()
    
    // Run a test (15 seconds max per phase)
    result, err := client.RunTest(ctx, 15*time.Second)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Download: %.2f Mbps\n", result.DownloadMbps)
    fmt.Printf("Upload: %.2f Mbps\n", result.UploadMbps)
    fmt.Printf("Latency: %.2f ms\n", result.LatencyMs)
}
```

### With Custom Configuration

```go
import (
    "log/slog"
    "net/http"
    "time"
    
    "github.com/yourusername/fastcom"
)

// Custom HTTP client with longer timeout
httpClient := &http.Client{
    Timeout: 60 * time.Second,
}

// Custom logger
logger := slog.Default()

// Create client with config
client := fastcom.NewClientWithConfig(fastcom.Config{
    HTTPClient: httpClient,
    Logger:     logger,
})

result, err := client.RunTest(ctx, 20*time.Second)
```

## API Reference

### Types

#### `Client`
The main client for running Fast.com speed tests.

#### `Result`
Contains the test results:
- `DownloadMbps float64` - Download speed in megabits per second
- `UploadMbps float64` - Upload speed in megabits per second
- `LatencyMs float64` - Unloaded ping latency in milliseconds
- `LoadedLatencyMs float64` - Latency during download (currently not measured)

#### `Config`
Configuration for the client:
- `HTTPClient *http.Client` - Custom HTTP client (optional)
- `Logger *slog.Logger` - Logger for debug messages (optional)

### Functions

#### `NewClient() *Client`
Creates a new client with default settings.

#### `NewClientWithConfig(cfg Config) *Client`
Creates a new client with custom configuration.

#### `(c *Client) RunTest(ctx context.Context, maxTime time.Duration) (*Result, error)`
Runs a complete speed test including download, upload, and ping measurements.
The `maxTime` parameter controls how long each test phase should run.

## How It Works

The library works by:

1. **Token Extraction**: Fetches the Fast.com homepage and extracts a token from the JavaScript file
2. **API Discovery**: Uses the token to fetch test URLs from Fast.com's API
3. **Download Test**: Performs concurrent GET requests to measure download speed
4. **Upload Test**: Performs concurrent POST requests with random data to measure upload speed
5. **Latency Test**: Performs TCP pings to measure network latency

## Limitations

- Latency measurement uses TCP connections rather than ICMP (raw sockets require special permissions)
- Loaded latency (latency during download) is not currently measured
- Jitter and packet loss metrics are not provided by Fast.com

## License

Same as the parent project.

