# dump1090 Exporter for Prometheus

Exports dump1090 ADS-B aircraft tracking statistics via HTTP for Prometheus.

## Features

- Exposes aircraft count metrics by directional sector
- Tracks maximum reception distance by compass direction
- Monitors total ADS-B messages received
- Supports both HTTP and filesystem data sources
- Multi-architecture Docker images (AMD64, ARM64)

## Usage

```
usage: dump1090_exporter [<flags>]

Flags:
  -h, --help                         Show context-sensitive help
      --version                      Show application version
      --web.listen-address=":9799"   Address on which to expose metrics
      --web.telemetry-path="/metrics" Path under which to expose metrics
      --web.disable-exporter-metrics Exclude metrics about the exporter itself
      --dump1090.address=URL         Address of dump1090 service (e.g. http://localhost:80/dump1090/data/)
      --dump1090.files=PATTERN       Location of dump1090 JSON files (e.g. /dev/shm/rbfeeder_%s)
      --compass.points="000,045,..."  Compass points for directional metrics
      --log.level=info               Log level (debug, info, warn, error)
```

Note: Either `--dump1090.address` or `--dump1090.files` must be supplied.

## Docker

Multi-architecture images are available via GitHub Container Registry:

```bash
docker pull ghcr.io/<username>/dump1090_exporter:latest
```

Run with rbfeeder:
```bash
docker run -d \
  -p 9799:9799 \
  -v /dev/shm:/dev/shm:ro \
  ghcr.io/<username>/dump1090_exporter:latest
```

## Metrics

- `dump1090_aircraft_count{with_position,direction}` - Number of aircraft in view
- `dump1090_aircraft_messages` - Total ADS-B messages received (counter)
- `dump1090_aircraft_timestamp` - Timestamp of last message
- `dump1090_aircraft_max_distance{direction}` - Maximum reception distance in meters

## Building

```bash
make build
# or
go build -o dump1090_exporter
```

## License

See LICENSE file.
