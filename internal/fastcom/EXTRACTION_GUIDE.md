# Extracting Fast.com Client to Standalone Library

This guide explains how to extract the `fastcom` package into its own Go module/library.

## Current Structure

The package is located at `internal/fastcom/` with the following files:
- `client.go` - Main client implementation
- `download.go` - Download speed test implementation
- `upload.go` - Upload speed test implementation
- `README.md` - Documentation
- `EXTRACTION_GUIDE.md` - This file

## Steps to Extract

### 1. Create New Repository

```bash
mkdir fastcom
cd fastcom
go mod init github.com/yourusername/fastcom
```

### 2. Copy Files

Copy all `.go` files from `internal/fastcom/` to the root of the new repository.

### 3. Update Package Name

The package name is already `fastcom`, so no changes needed.

### 4. Update Imports (if any)

The package only uses standard library imports, so no external dependencies need to be added to `go.mod`.

### 5. Add Examples

Create an `examples/` directory with usage examples:

```go
// examples/basic/main.go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/yourusername/fastcom"
)

func main() {
    ctx := context.Background()
    client := fastcom.NewClient()
    result, err := client.RunTest(ctx, 15*time.Second)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Download: %.2f Mbps\n", result.DownloadMbps)
    fmt.Printf("Upload: %.2f Mbps\n", result.UploadMbps)
    fmt.Printf("Latency: %.2f ms\n", result.LatencyMs)
}
```

### 6. Add Tests

Create test files:
- `client_test.go`
- `download_test.go`
- `upload_test.go`

### 7. Update Documentation

Update the README.md with the new module path and any additional information.

### 8. Update Parent Project

In the `internet-perf-exporter` project:

1. Remove `internal/fastcom/` directory
2. Update `go.mod` to include the new module:
   ```bash
   go get github.com/yourusername/fastcom@latest
   ```
3. Update imports in `internal/collectors/fast_collector.go`:
   ```go
   import "github.com/yourusername/fastcom"
   ```

## Dependencies

The package has **zero external dependencies** - it only uses:
- Standard library (`context`, `crypto/rand`, `encoding/json`, `fmt`, `io`, `log/slog`, `net`, `net/http`, `net/url`, `regexp`, `strings`, `sync`, `time`)

## Version Compatibility

- Go 1.21+ (for `log/slog`)

For Go versions < 1.21, you could either:
- Remove logging support
- Use a logging interface instead of `slog.Logger`
- Add a dependency on a logging library

## Future Enhancements

Potential improvements if extracted:
- Add benchmarks
- Add more configuration options (connection count, payload sizes)
- Support for IPv6-only testing
- Support for loaded latency measurement
- Progress callbacks for long-running tests

