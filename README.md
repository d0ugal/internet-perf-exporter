# Internet Performance Exporter

A Prometheus exporter for monitoring internet performance metrics using multiple backends (speedtest, fast.com, and more).

**Image**: `ghcr.io/d0ugal/internet-perf-exporter:latest`

## Features

- **Multiple Backends**: Support for Speedtest.net and Fast.com (Netflix)
- **Comprehensive Metrics**: Download/upload speeds, latency, jitter, packet loss
- **Flexible Configuration**: YAML files or environment variables
- **OpenTelemetry Tracing**: Optional distributed tracing support
- **Production Ready**: Docker support, health checks, graceful shutdown

## Metrics

### Speed Metrics
- `internet_perf_exporter_download_speed_mbps`: Download speed in Mbps
- `internet_perf_exporter_upload_speed_mbps`: Upload speed in Mbps

### Latency Metrics
- `internet_perf_exporter_latency_ms`: Latency in milliseconds (histogram)
- `internet_perf_exporter_jitter_ms`: Jitter in milliseconds
- `internet_perf_exporter_packet_loss_percent`: Packet loss percentage (0-100)

### Test Metrics
- `internet_perf_exporter_test_duration_seconds`: Duration of speed test in seconds (histogram)
- `internet_perf_exporter_test_success`: Test success status (1=success, 0=failure)
- `internet_perf_exporter_test_failed_total`: Total number of failed tests

### Server Info
- `internet_perf_exporter_server_info`: Information about the test server

### Collection Metrics
- `internet_perf_exporter_collection_duration_seconds`: Duration of collection in seconds
- `internet_perf_exporter_collection_success_total`: Total number of successful collections
- `internet_perf_exporter_collection_failed_total`: Total number of failed collections

All metrics include labels:
- `backend`: Backend name (speedtest, fast)
- `server_id`: Server identifier
- `server_location`: Server location/name

### Endpoints
- `GET /`: HTML dashboard with service status and metrics information
- `GET /metrics`: Prometheus metrics endpoint
- `GET /health`: Health check endpoint

## Quick Start

### Docker Compose

```yaml
version: '3.8'
services:
  internet-perf-exporter:
    image: ghcr.io/d0ugal/internet-perf-exporter:latest
    ports:
      - "8080:8080"
    environment:
      - INTERNET_PERF_EXPORTER_CONFIG_FROM_ENV=true
      - INTERNET_PERF_EXPORTER_SPEEDTEST_ENABLED=true
      - INTERNET_PERF_EXPORTER_SPEEDTEST_INTERVAL=1h
      - INTERNET_PERF_EXPORTER_FAST_ENABLED=true
      - INTERNET_PERF_EXPORTER_FAST_INTERVAL=1h
    restart: unless-stopped
```

1. Run: `docker-compose up -d`
2. Access metrics: `curl http://localhost:8080/metrics`

## Configuration

### YAML Configuration

Create a `config.yaml` file:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

logging:
  level: "info"
  format: "json"

metrics:
  collection:
    default_interval: "5m"

backends:
  speedtest:
    type: "speedtest"
    enabled: true
    interval: "1h"
    timeout: "5m"
    speedtest:
      server_id: 12345  # Optional: specific server ID
      # server_country: "US"  # Optional: filter by country
      # server_name: "Example ISP"  # Optional: filter by name

  fast:
    type: "fast"
    enabled: true
    interval: "1h"
    timeout: "5m"
```

### Environment Variables

All configuration can be provided via environment variables:

```bash
# Server configuration
INTERNET_PERF_EXPORTER_SERVER_HOST=0.0.0.0
INTERNET_PERF_EXPORTER_SERVER_PORT=8080

# Logging
INTERNET_PERF_EXPORTER_LOGGING_LEVEL=info
INTERNET_PERF_EXPORTER_LOGGING_FORMAT=json

# Speedtest backend
INTERNET_PERF_EXPORTER_SPEEDTEST_ENABLED=true
INTERNET_PERF_EXPORTER_SPEEDTEST_INTERVAL=1h
INTERNET_PERF_EXPORTER_SPEEDTEST_TIMEOUT=5m
INTERNET_PERF_EXPORTER_SPEEDTEST_SERVER_ID=12345  # Optional

# Fast.com backend
INTERNET_PERF_EXPORTER_FAST_ENABLED=true
INTERNET_PERF_EXPORTER_FAST_INTERVAL=1h
INTERNET_PERF_EXPORTER_FAST_TIMEOUT=5m

# Tracing (optional)
TRACING_ENABLED=true
TRACING_SERVICE_NAME=internet-perf-exporter
TRACING_ENDPOINT=http://localhost:4318/v1/traces
```

## Backends

### Speedtest.net

Uses the [github.com/showwin/speedtest-go](https://github.com/showwin/speedtest-go) library to test against Speedtest.net servers.

**Features:**
- Download and upload speed testing
- Latency (ping) testing
- Server selection by ID, name, or country
- Automatic selection of closest server

**Configuration:**
- `server_id`: Specific server ID (0 = auto-select)
- `server_name`: Filter by server name
- `server_country`: Filter by server country code

### Fast.com

Uses the [gopkg.in/ddo/go-fast.v0](https://github.com/ddo/go-fast) library to test against Netflix's Fast.com service.

**Features:**
- Download speed testing
- Simple and fast measurements
- No server selection needed

**Note:** Fast.com only measures download speed, not upload or latency.

## Deployment

### Docker Compose

See Quick Start section above.

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: internet-perf-exporter
spec:
  replicas: 1
  selector:
    matchLabels:
      app: internet-perf-exporter
  template:
    metadata:
      labels:
        app: internet-perf-exporter
    spec:
      containers:
      - name: internet-perf-exporter
        image: ghcr.io/d0ugal/internet-perf-exporter:latest
        ports:
        - containerPort: 8080
        env:
        - name: INTERNET_PERF_EXPORTER_CONFIG_FROM_ENV
          value: "true"
        - name: INTERNET_PERF_EXPORTER_SPEEDTEST_ENABLED
          value: "true"
        - name: INTERNET_PERF_EXPORTER_SPEEDTEST_INTERVAL
          value: "1h"
```

## Use Cases

- **ISP Monitoring**: Track connection quality and performance over time
- **Network Troubleshooting**: Identify speed degradation or latency spikes
- **Service Level Monitoring**: Monitor internet connectivity for critical services
- **Alerting**: Set up Prometheus alerts for performance degradation
- **Grafana Dashboards**: Visualize internet performance trends

## Example Queries

### Average download speed by backend
```promql
avg(internet_perf_exporter_download_speed_mbps) by (backend)
```

### Latency percentile
```promql
histogram_quantile(0.95, internet_perf_exporter_latency_ms)
```

### Test success rate
```promql
rate(internet_perf_exporter_collection_success_total[5m]) / 
rate(internet_perf_exporter_collection_success_total[5m] + internet_perf_exporter_collection_failed_total[5m])
```

## Development

### Build

```bash
make build
```

### Run Tests

```bash
make test
```

### Lint

```bash
make lint
```

## License

MIT License - see [LICENSE](LICENSE) file for details.

