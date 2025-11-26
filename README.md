# Docker Container Monitor 🐳

A powerful Docker Container Monitoring Tool built with Go. Shows real-time CPU, memory, network, and IO usage using the Docker Engine API, with an optional web dashboard and REST API. Ideal for DevOps automation and container observability.

## Features

- **Real-time Monitoring**: Track CPU, memory, network, and disk I/O statistics for all running containers
- **Multiple Modes**: 
  - CLI mode for terminal-based monitoring
  - Web Dashboard for visual monitoring
  - REST API for programmatic access
- **Docker Engine API**: Direct integration with Docker for accurate statistics
- **Lightweight**: Minimal resource footprint
- **DevOps Ready**: Perfect for automation scripts and monitoring pipelines

## Prerequisites

- Go 1.24 or higher
- Docker Engine installed and running
- Access to Docker socket (typically requires running as root or being in the docker group)

## Installation

### From Source

```bash
git clone https://github.com/shlok38/Docker_monitor-.git
cd Docker_monitor-
go build -o docker-monitor
```

### Using Docker

```bash
# Build the Docker image
docker build -t docker-monitor .

# Run the container
docker run -d \
  --name docker-monitor \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  docker-monitor
```

### Using Docker Compose

```bash
# Clone and start
git clone https://github.com/shlok38/Docker_monitor-.git
cd Docker_monitor-
docker-compose up -d
```

### Direct Run

```bash
go run main.go
```

## Usage

### CLI Mode (Default)

Monitor containers in your terminal with real-time updates:

```bash
./docker-monitor
```

**Options:**
- `-interval`: Update interval in seconds (default: 2)

```bash
./docker-monitor -interval 5
```

### Web Dashboard Mode

Start the web dashboard with REST API:

```bash
./docker-monitor -api
```

**Options:**
- `-port`: HTTP server port (default: 8080)

```bash
./docker-monitor -api -port 3000
```

Then open your browser to:
- Dashboard: http://localhost:8080
- API Endpoint: http://localhost:8080/api/stats

### REST API

The REST API provides JSON data for all running containers:

**Endpoint:** `GET /api/stats`

**Response Example:**
```json
[
  {
    "id": "a1b2c3d4e5f6",
    "name": "/my-container",
    "cpu_percent": 2.45,
    "memory_usage": 104857600,
    "memory_limit": 2147483648,
    "memory_percent": 4.88,
    "network_rx": 1048576,
    "network_tx": 524288,
    "block_read": 2097152,
    "block_write": 1048576
  }
]
```

**Use with curl:**
```bash
curl http://localhost:8080/api/stats
```

**Use with jq for pretty output:**
```bash
curl -s http://localhost:8080/api/stats | jq
```

## Metrics Explained

| Metric | Description |
|--------|-------------|
| **CPU %** | Percentage of CPU usage relative to the host system |
| **Memory Usage** | Current memory consumption / Memory limit |
| **Memory %** | Percentage of allocated memory being used |
| **Network I/O** | Bytes received / Bytes transmitted over network |
| **Block I/O** | Bytes read / Bytes written to disk |

## Screenshots

### CLI Mode
```
Docker Container Monitor
========================
Time: 2025-11-26 10:00:00

CONTAINER ID    NAME                           CPU %       MEMORY              MEM %    NET I/O         BLOCK I/O
-----------------------------------------------------------------------------------------------------------------------------------
a1b2c3d4e5f6    /redis-server                  2.45%      256.00 MiB / 2 GiB  12.50%   1.5 MiB / 512 KiB   2 MiB / 1 MiB
```

### Web Dashboard
The web dashboard provides a modern, responsive interface with:
- Real-time auto-refresh every 2 seconds
- Beautiful gradient design
- Progress bars for CPU and memory usage
- Network and disk I/O statistics
- Mobile-friendly responsive layout

## Docker Permissions

To access Docker socket, you may need to:

```bash
# Add your user to docker group
sudo usermod -aG docker $USER

# Or run with sudo
sudo ./docker-monitor
```

## Use Cases

### DevOps Automation
Integrate with monitoring pipelines:
```bash
# Get JSON stats for processing
curl -s http://localhost:8080/api/stats | jq '.[] | select(.cpu_percent > 80)'
```

### Container Health Monitoring
Monitor specific containers in scripts:
```bash
# Alert if any container exceeds memory threshold
curl -s http://localhost:8080/api/stats | \
  jq '.[] | select(.memory_percent > 90) | .name'
```

### Performance Tracking
Use in CI/CD pipelines to track container resource usage during tests.

### Integration with Docker Compose
Create a monitoring service in your docker-compose.yml:
```yaml
version: '3.8'
services:
  monitor:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    command: ["-api", "-port", "8080"]
```

Create a simple Dockerfile:
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o docker-monitor

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/docker-monitor .
EXPOSE 8080
ENTRYPOINT ["./docker-monitor"]
```

## Development

### Project Structure
```
.
├── main.go           # Main application code
├── go.mod            # Go module definition
├── go.sum            # Go module checksums
└── README.md         # This file
```

### Building
```bash
go build -o docker-monitor
```

### Testing
Ensure Docker is running and you have some containers:
```bash
# Start test containers
docker run -d --name test-nginx nginx
docker run -d --name test-redis redis

# Run the monitor
./docker-monitor
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is open source and available under the MIT License.

## Acknowledgments

- Built with the [Docker Go SDK](https://github.com/docker/docker)
- Inspired by `docker stats` command

## Support

For issues, questions, or contributions, please visit the [GitHub repository](https://github.com/shlok38/Docker_monitor-).
