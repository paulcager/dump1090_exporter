# dump1090_exporter

## Overview
A Prometheus exporter for dump1090 ADS-B aircraft tracking data. This exporter collects statistics from dump1090 JSON files and exposes them as Prometheus metrics.

## Purpose
Provides visibility into aircraft tracking performance by exposing metrics about:
- Number of aircraft currently in view
- Total ADS-B messages received
- Maximum reception distance by compass direction
- Aircraft count by directional sector

## Architecture

### Main Components
- **dump1090_exporter.go**: Main exporter implementation
  - Prometheus exporter that implements the `Collect()` and `Describe()` interfaces
  - Fetches `aircraft.json` and `receiver.json` from dump1090
  - Calculates distances and bearings using receiver position
  - Exports metrics by compass sector (configurable, defaults to 45° sectors)

- **sectors.go**: Utility functions for compass sector calculations

### Data Sources
The exporter supports two modes:
1. **File-based** (`--dump1090.files`): Read JSON files from filesystem (e.g., `/dev/shm/rbfeeder_%s`)
2. **HTTP-based** (`--dump1090.address`): Fetch JSON from dump1090 web interface

### Metrics Exposed
- `dump1090_aircraft_count{with_position,direction}`: Number of aircraft in view, segmented by sector
- `dump1090_aircraft_messages`: Total ADS-B messages received
- `dump1090_aircraft_timestamp`: Timestamp of last message
- `dump1090_aircraft_max_distance{direction}`: Maximum reception distance in meters by sector

### Configuration
Key command-line flags:
- `--web.listen-address`: Prometheus metrics endpoint (default: `:9799`)
- `--web.telemetry-path`: Metrics path (default: `/metrics`)
- `--web.disable-exporter-metrics`: Exclude Go runtime metrics
- `--dump1090.address`: HTTP URL to dump1090 data
- `--dump1090.files`: Filesystem path pattern to JSON files
- `--compass.points`: Compass sectors for directional metrics (default: `000,045,090,135,180,225,270,315`)

## Dependencies
- `github.com/prometheus/*`: Prometheus client libraries
- `github.com/paulcager/osgridref`: Distance and bearing calculations
- `github.com/go-kit/log`: Structured logging

## Docker Build
Multi-stage build:
1. **Build stage**: Uses `paulcager/go-base:latest`, compiles Go binary with CGO disabled
2. **Runtime stage**: Minimal `scratch` image with just the binary and CA certificates
3. **Default configuration**: Configured for rbfeeder files at `/dev/shm/rbfeeder_%s`

## Build & Deployment
Built using GitHub Actions workflow that creates multi-architecture images:
- Platforms: `linux/amd64`, `linux/arm64`
- Published to: `ghcr.io/<owner>/dump1090_exporter`
- Triggers: Push to main/master, PRs, manual workflow dispatch

## Integration Points
- **Input**: dump1090 JSON files (`aircraft.json`, `receiver.json`)
- **Output**: Prometheus metrics on port 9799
- **Common usage**: Monitoring rbfeeder or dump1090 installations

## Development Notes
- Uses `osgridref` package for precise distance/bearing calculations
- Supports configurable compass sectors for directional analysis
- Optional exclusion of exporter metrics for minimal overhead
- Designed to run as unprivileged user (warns if running as root)
