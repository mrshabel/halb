# HALB - Highly Available Load Balancer

HALB is a layer-7 load balancer that routes traffic between multiple upstream servers.
My motivation for building this tool was to understand routing patterns at the application layer between proxies and servers using HTTP/1.1 protocol.
I hope you find this tool useful

## Features

- Self-healing backends - Active health checks automatically remove unhealthy backends.
- Hot reload - Edit config and apply instantly with zero downtime and no lock contention.
- Lock-free routing - Uses atomic variables in the hot path to scale with CPU cores.
- Multiple strategies - Supports Round Robin and Least Connections.

## Quick Start

### Docker

```bash
# Pull and run with default config
docker run -p 8000:8000 mrshabel/halb

# With custom config
docker run -p 8000:8000 -v /path/to/config.yaml:/app/config.yaml mrshabel/halb
```

### Install Script

```bash
# Linux/macOS
curl -sL https://raw.githubusercontent.com/mrshabel/halb/main/install.sh | sh

# Or with wget
wget -qO- https://raw.githubusercontent.com/mrshabel/halb/main/install.sh | sh
```

### Build from Source

```bash
git clone https://github.com/mrshabel/halb.git
cd halb
make build
./bin/halb
```

This starts a single-node HALB instance using the default configuration found in `configs/config.yaml`.
You can optionally specify a custom config file: `./bin/halb -config my-config.yaml`

## Configuration

A single yaml file is all you need to enable load balancing for your backend servers
`configs/config.yaml`:

```yaml
server:
    port: 8000
    timeout: 30s

services:
    # auth service (round_robin default)
    auth:
        host: auth.localhost
        servers:
            - http://localhost:9001
            - http://localhost:9002
        health:
            path: /health
            interval: 5s

    # api service (least connections)
    api:
        host: api.localhost
        strategy: least_conn
        servers:
            - http://localhost:9003
        health:
            path: /health
            interval: 5s
```

Hot Reload: Edit and save this file while the server is running. HALB reloads the routing table automatically without dropping connections.

## Docker Compose Example

Create a `docker-compose.yaml` file:

```yaml
services:
    halb:
        image: mrshabel/halb
        ports:
            - "8000:8000"
        volumes:
            - ./config.yaml:/app/config.yaml:ro
        restart: unless-stopped

    backend-1:
        image: nginx:alpine
        healthcheck:
            test: ["CMD", "wget", "-q", "--spider", "http://localhost/health"]
            interval: 5s
            timeout: 3s
            retries: 2

    backend-2:
        image: nginx:alpine
        healthcheck:
            test: ["CMD", "wget", "-q", "--spider", "http://localhost/health"]
            interval: 5s
            timeout: 3s
            retries: 2
```

Then run:

```bash
docker compose up -d
```

## Development

Start the test servers and the HALB load balancer:

```bash
make dev
```

In another terminal:

```bash
# test round robin
curl http://auth.localhost:8000
curl http://auth.localhost:8000
curl http://auth.localhost:8000
```

## Technical Considerations

- HTTP keepalives are enabled by default to ensure maximum performance while reducing TCP handshake overhead
- Atomic variables are used in the hot path (routing) to avoid lock contention at scale
- A routing table containing the configuration defined is stored where a reload triggers an atomic swap pointer operation, allowing readers to continue processing with old config until a new one is swapped in.
- Active health checks are enforced to prevent routing requests to dead backends.
- To reduce the chances of false positives when detecting dead backends, 3 successive health checks need to be recorded before it is marked as unhealthy.
  Same applies for when a backend is dead and needs to transition into a healthy state.
- Health check intervals must be short to minimize 502 errors during failover
