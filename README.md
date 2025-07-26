# FlowGuard - Smart API Rate-Limiting Reverse Proxy

FlowGuard is a high-performance, Go-based reverse proxy that provides intelligent API rate limiting with both requests per minute (RPM) and tokens per minute (TPM) controls. It includes comprehensive monitoring, real-time configuration APIs, and a beautiful Grafana dashboard for observability.

## 🚀 Features

- **Dual Rate Limiting**: Both RPM (requests/minute) and TPM (tokens/minute) limits per client
- **Token Bucket Algorithm**: Smooth rate limiting with burst capability
- **Real-time Configuration**: REST and gRPC APIs for live configuration updates
- **Comprehensive Monitoring**: Prometheus metrics with pre-built Grafana dashboard
- **Header-based Client Identification**: Uses `X-Client-ID` and `X-Token-Estimate` headers
- **Containerized Deployment**: Full Docker Compose stack with Prometheus and Grafana
- **Thread-safe Operations**: Concurrent request handling with proper synchronization
- **Graceful Error Handling**: Detailed error responses and logging
- **Auto-generated Protobuf**: gRPC files generated during build process

## 📋 Requirements

- Docker and Docker Compose
- Go 1.21+ (for local development)
- Optional: grpcurl for gRPC testing

## 🏃 Quick Start

### 1. Clone and Start the Stack

```bash
# Clone the repository
git clone <repository-url>
cd flowguard

# Start the complete stack (protobuf files auto-generated during build)
docker-compose up --build
```

### 2. Access the Services

- **FlowGuard Proxy**: http://localhost:8080 (API traffic)
- **FlowGuard Metrics**: http://localhost:9090/metrics (Prometheus metrics)
- **FlowGuard REST API**: http://localhost:9091/api/v1 (Configuration)
- **FlowGuard gRPC**: localhost:9092 (gRPC configuration)
- **Prometheus**: http://localhost:9093 (Metrics collection)
- **Grafana**: http://localhost:3000 (Dashboard - admin/admin)

## 🔧 Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `UPSTREAM_URL` | `https://api.openai.com` | Target API to proxy to |
| `PROXY_PORT` | `8080` | Proxy server port |
| `METRICS_PORT` | `9090` | Metrics endpoint port |
| `CONFIG_PORT` | `9091` | REST API port |
| `GRPC_PORT` | `9092` | gRPC server port |

### Default Clients

FlowGuard comes with pre-configured demo clients:

- `demo-client`: 60 RPM, 1000 TPM
- `test-client`: 30 RPM, 500 TPM  
- `premium-client`: 120 RPM, 2000 TPM

## 📡 API Usage

### Making Proxied Requests

All requests to the proxy endpoint must include two headers:

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "X-Client-ID: demo-client" \
  -H "X-Token-Estimate: 150" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-3.5-turbo", "messages": [{"role": "user", "content": "Hello!"}]}'
```

**Required Headers:**
- `X-Client-ID`: Unique identifier for the client
- `X-Token-Estimate`: Number of tokens this request will consume (integer)

### REST API Examples

#### Get all clients

```bash
curl http://localhost:9091/api/v1/clients
```

#### Create/Update client configuration

```bash
curl -X POST http://localhost:9091/api/v1/clients \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "my-client",
    "rpm": 100,
    "tpm": 2000,
    "enabled": true
  }'
```

#### Get specific client

```bash
curl http://localhost:9091/api/v1/clients/my-client
```

#### Update client configuration

```bash
curl -X PUT http://localhost:9091/api/v1/clients/my-client \
  -H "Content-Type: application/json" \
  -d '{
    "rpm": 200,
    "tpm": 5000,
    "enabled": true
  }'
```

#### Delete client

```bash
curl -X DELETE http://localhost:9091/api/v1/clients/my-client
```

#### Get client statistics

```bash
curl http://localhost:9091/api/v1/clients/my-client/stats
```

#### Get all statistics

```bash
curl http://localhost:9091/api/v1/stats
```

#### Health check

```bash
curl http://localhost:9091/health
```

### gRPC API Examples

#### Install grpcurl (if not installed)

```bash
# macOS
brew install grpcurl

# Linux
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

#### List available services

```bash
grpcurl -plaintext localhost:9092 list
```

#### Set client configuration

```bash
grpcurl -plaintext -d '{
  "config": {
    "client_id": "grpc-client",
    "rpm": 150,
    "tpm": 3000,
    "enabled": true
  }
}' localhost:9092 flowguard.FlowGuardService/SetClientConfig
```

#### Get client configuration

```bash
grpcurl -plaintext -d '{
  "client_id": "grpc-client"
}' localhost:9092 flowguard.FlowGuardService/GetClientConfig
```

#### Get client statistics

```bash
grpcurl -plaintext -d '{
  "client_id": "grpc-client"
}' localhost:9092 flowguard.FlowGuardService/GetClientStats
```

#### List all clients

```bash
grpcurl -plaintext -d '{}' localhost:9092 flowguard.FlowGuardService/ListClients
```

#### Delete client

```bash
grpcurl -plaintext -d '{
  "client_id": "grpc-client"
}' localhost:9092 flowguard.FlowGuardService/DeleteClient
```

## 📊 Monitoring

### Prometheus Metrics

FlowGuard exposes the following metrics:

- `flowguard_requests_total`: Total requests processed
- `flowguard_requests_dropped_total`: Requests dropped due to rate limiting
- `flowguard_tokens_used_total`: Total tokens consumed
- `flowguard_tokens_remaining`: Current tokens remaining in buckets
- `flowguard_request_duration_milliseconds`: Request latency histogram
- `flowguard_rate_limit_remaining`: Current rate limit remaining

### Grafana Dashboard

The included Grafana dashboard provides:

- Request rate by client
- Drop rate by client and reason (RPM/TPM)
- Rate limit tokens remaining
- Request latency percentiles
- Token consumption rate

Access at http://localhost:3000 (admin/admin)

### Example Prometheus Queries

```promql
# Request rate per client
rate(flowguard_requests_total[5m])

# Drop rate percentage
rate(flowguard_requests_dropped_total[5m]) / rate(flowguard_requests_total[5m]) * 100

# 95th percentile latency
histogram_quantile(0.95, rate(flowguard_request_duration_milliseconds_bucket[5m]))

# Token consumption rate
rate(flowguard_tokens_used_total[5m])
```

## 🧪 Testing Rate Limits

### Test RPM limit

```bash
# Send rapid requests to trigger RPM limit
for i in {1..100}; do
  curl -s -X GET http://localhost:8080/test \
    -H "X-Client-ID: test-client" \
    -H "X-Token-Estimate: 1" &
done
wait
```

### Test TPM limit

```bash
# Send high token estimate to trigger TPM limit
curl -X GET http://localhost:8080/test \
  -H "X-Client-ID: test-client" \
  -H "X-Token-Estimate: 600"  # Exceeds 500 TPM limit
```

### Load Testing Script

Create `test_load.sh`:

```bash
#!/bin/bash

CLIENT_ID=${1:-"demo-client"}
REQUESTS=${2:-50}
TOKENS=${3:-100}

echo "Testing $CLIENT_ID with $REQUESTS requests, $TOKENS tokens each"

for i in $(seq 1 $REQUESTS); do
  response=$(curl -s -w "%{http_code}" -o /dev/null \
    -X GET http://localhost:8080/test \
    -H "X-Client-ID: $CLIENT_ID" \
    -H "X-Token-Estimate: $TOKENS")
  
  echo "Request $i: HTTP $response"
  sleep 0.1
done
```

Run it:

```bash
chmod +x test_load.sh
./test_load.sh demo-client 20 50
```

## 🏗️ Development

### Local Development Setup

```bash
# Install dependencies
go mod tidy

# Generate protobuf files (for local development)
export PATH=$PATH:$(go env GOPATH)/bin
protoc --go_out=internal --go_opt=paths=source_relative \
       --go-grpc_out=internal --go-grpc_opt=paths=source_relative \
       proto/flowguard.proto

# Run locally
go run cmd/flowguard/main.go
```

### Project Structure

```
flowguard/
├── cmd/flowguard/main.go           # Main application entry point
├── internal/
│   ├── types/types.go              # Core types and structures
│   ├── limiter/manager.go          # Rate limiting logic
│   ├── proxy/handler.go            # Reverse proxy implementation
│   ├── config/rest.go              # REST API handlers
│   ├── config/grpc.go              # gRPC server implementation
│   ├── metrics/prometheus.go       # Prometheus metrics
│   └── proto/                      # Generated protobuf code (auto-generated)
├── proto/flowguard.proto           # gRPC service definition
├── Dockerfile                      # Multi-stage Docker build with protobuf generation
├── docker-compose.yml              # Complete stack orchestration
├── prometheus/prometheus.yml       # Prometheus configuration
├── grafana/
│   ├── dashboards/                 # Pre-built dashboards
│   └── provisioning/               # Grafana auto-configuration
├── .gitignore                      # Ignore generated files and build artifacts
└── README.md                       # This file
```

### Building

```bash
# Build binary (requires protobuf files to be generated first)
go build -o flowguard cmd/flowguard/main.go

# Build Docker image (auto-generates protobuf files)
docker build -t flowguard .

# Build and run with compose (recommended)
docker-compose up --build
```

### Protobuf Generation

**Note**: Protobuf files (`*.pb.go` and `*_grpc.pb.go`) are automatically generated during the Docker build process and should NOT be committed to version control.

For local development:
```bash
# Install protoc and Go plugins
brew install protobuf  # macOS
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate files
export PATH=$PATH:$(go env GOPATH)/bin
protoc --go_out=internal --go_opt=paths=source_relative \
       --go-grpc_out=internal --go-grpc_opt=paths=source_relative \
       proto/flowguard.proto
```

## 🔧 Advanced Configuration

### Custom Rate Limits

```json
{
  "client_id": "custom-client",
  "rpm": null,        // No RPM limit
  "tpm": 5000,        // Only TPM limit
  "enabled": true
}
```

### Disable Rate Limiting for a Client

```json
{
  "client_id": "unlimited-client",
  "enabled": false
}
```

### Environment Variables for Docker

Create `.env` file:

```env
UPSTREAM_URL=https://your-api.com
PROXY_PORT=8080
METRICS_PORT=9090
CONFIG_PORT=9091
GRPC_PORT=9092
```

Use with docker-compose:

```bash
docker-compose --env-file .env up
```

## 🚨 Error Responses

FlowGuard returns structured JSON error responses:

### Missing Headers (400)

```json
{
  "error": "missing_header",
  "message": "X-Client-ID header is required"
}
```

### Invalid Token Estimate (400)

```json
{
  "error": "invalid_header", 
  "message": "X-Token-Estimate must be a non-negative integer"
}
```

### RPM Limit Exceeded (429)

```json
{
  "error": "rpm_exceeded",
  "message": "Request rate limit exceeded"
}
```

### TPM Limit Exceeded (429)

```json
{
  "error": "tpm_exceeded",
  "message": "Token rate limit exceeded"
}
```

## 🎯 Performance

- **Throughput**: Handles thousands of concurrent requests
- **Latency**: Sub-millisecond rate limit decisions
- **Memory**: Efficient token bucket implementation
- **CPU**: Optimized for high-frequency operations

## 🔐 Security

- No authentication/authorization (add via reverse proxy if needed)
- Rate limiting based on client-provided headers
- CORS headers configurable
- Secure defaults for production deployment

## 🐛 Troubleshooting

### Common Issues

1. **Port conflicts**: Check if ports 8080, 9090, 9091, 9092, 3000, 9093 are available
2. **Docker permission issues**: Ensure Docker daemon is running
3. **Missing headers**: All proxy requests require `X-Client-ID` and `X-Token-Estimate`
4. **Grafana dashboard not loading**: Wait for Prometheus to collect initial metrics
5. **Protobuf compilation errors**: Ensure Docker has internet access to install build tools

### Logs

```bash
# View FlowGuard logs
docker-compose logs flowguard

# View all logs
docker-compose logs

# Follow logs
docker-compose logs -f flowguard
```

### Debug Mode

Set environment variable for more verbose logging:

```bash
export LOG_LEVEL=debug
```
