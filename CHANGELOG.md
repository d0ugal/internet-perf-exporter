# Changelog

## [0.1.4](https://github.com/d0ugal/internet-perf-exporter/compare/v0.1.3...v0.1.4) (2025-11-05)


### Bug Fixes

* update dependency go to v1.25.4 ([b63a16f](https://github.com/d0ugal/internet-perf-exporter/commit/b63a16f873be8a04d947847c6f6f2e236e847b55))

## [0.1.3](https://github.com/d0ugal/internet-perf-exporter/compare/v0.1.2...v0.1.3) (2025-11-05)


### Bug Fixes

* update module github.com/d0ugal/promexporter to v1.11.0 ([#27](https://github.com/d0ugal/internet-perf-exporter/issues/27)) ([a7617b5](https://github.com/d0ugal/internet-perf-exporter/commit/a7617b5ebba298fa67bc8b4a8c5379fe177e5e39))

## [0.1.2](https://github.com/d0ugal/internet-perf-exporter/compare/v0.1.1...v0.1.2) (2025-11-04)


### Bug Fixes

* update google.golang.org/genproto/googleapis/api digest to f26f940 ([a9b06b0](https://github.com/d0ugal/internet-perf-exporter/commit/a9b06b07d1c22a698a44c322f281c9c5098d0d60))
* update google.golang.org/genproto/googleapis/api digest to f26f940 ([#19](https://github.com/d0ugal/internet-perf-exporter/issues/19)) ([08bf318](https://github.com/d0ugal/internet-perf-exporter/commit/08bf3189504477bddcb7cb6157fb49809106e17b))
* update google.golang.org/genproto/googleapis/rpc digest to f26f940 ([#20](https://github.com/d0ugal/internet-perf-exporter/issues/20)) ([8027860](https://github.com/d0ugal/internet-perf-exporter/commit/802786038922a22b913e6622088611616f7816ad))
* update module github.com/d0ugal/promexporter to v1.9.0 ([d2c30d4](https://github.com/d0ugal/internet-perf-exporter/commit/d2c30d41ae83ed175e69144e824d98c9b8f297ba))
* update module go.opentelemetry.io/proto/otlp to v1.9.0 ([a6154a6](https://github.com/d0ugal/internet-perf-exporter/commit/a6154a6b84873b7a846597f9ef285dd17505f3e8))

## [0.1.1](https://github.com/d0ugal/internet-perf-exporter/compare/v0.1.0...v0.1.1) (2025-11-02)


### Bug Fixes

* **speedtest:** always export jitter and packet loss metrics ([#17](https://github.com/d0ugal/internet-perf-exporter/issues/17)) ([ec933d8](https://github.com/d0ugal/internet-perf-exporter/commit/ec933d8337abb4b7fe77e32d34504c52bb9d8f3f))

## [0.1.0](https://github.com/d0ugal/internet-perf-exporter/compare/v0.0.1...v0.1.0) (2025-11-02)


### Features

* add test coordinator to prevent concurrent tests ([d81a9db](https://github.com/d0ugal/internet-perf-exporter/commit/d81a9dbcecc4a985e3e351d219a556c4c07ebf67))
* add test coordinator to prevent concurrent tests ([52c7105](https://github.com/d0ugal/internet-perf-exporter/commit/52c710551b99ffb1236fbcb577c737f57fac5b30))
* **fastcom:** add OpenTelemetry HTTP client instrumentation and operation spans ([75e63de](https://github.com/d0ugal/internet-perf-exporter/commit/75e63de5758b970922b706672be9761d17d2bada))
* implement custom Fast.com client with upload and latency support ([#11](https://github.com/d0ugal/internet-perf-exporter/issues/11)) ([f8cefc3](https://github.com/d0ugal/internet-perf-exporter/commit/f8cefc3a544880e6558552293b80270b19262e75))


### Bug Fixes

* check error return values from Close() calls ([#12](https://github.com/d0ugal/internet-perf-exporter/issues/12)) ([91fc36f](https://github.com/d0ugal/internet-perf-exporter/commit/91fc36faf7cd3ad6e76d3057bd96d7d90e945ce4))
* improve speedtest accuracy with proper ByteRate conversion and context support ([47ccef9](https://github.com/d0ugal/internet-perf-exporter/commit/47ccef97597007724aaa90b6cc1c84c77a6719b7))
* improve speedtest accuracy with proper ByteRate conversion and context support ([7d0a172](https://github.com/d0ugal/internet-perf-exporter/commit/7d0a1726d2f30c1c455d44354a465b96e0b76489))
* lint ([2caec82](https://github.com/d0ugal/internet-perf-exporter/commit/2caec826c6bc7b2c95b7e40358849e2c1af71030))
* update module go.yaml.in/yaml/v2 to v3 ([3957ad8](https://github.com/d0ugal/internet-perf-exporter/commit/3957ad8cb54008cb748b6c15780cc52018bf631b))
* update module go.yaml.in/yaml/v2 to v3 ([#1](https://github.com/d0ugal/internet-perf-exporter/issues/1)) ([9aeb17d](https://github.com/d0ugal/internet-perf-exporter/commit/9aeb17d3202982ecf48e959f969381867f10731d))
* update module gopkg.in/ddo/go-dlog.v1 to v2 ([#2](https://github.com/d0ugal/internet-perf-exporter/issues/2)) ([cb6f43d](https://github.com/d0ugal/internet-perf-exporter/commit/cb6f43de3113d42a50f147a5cdd1d3e1933fff47))
* update module gopkg.in/ddo/request.v1 to v2 ([#7](https://github.com/d0ugal/internet-perf-exporter/issues/7)) ([c6cbc95](https://github.com/d0ugal/internet-perf-exporter/commit/c6cbc955037da2ee703cf1d800434de7ed62105f))
