# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git protobuf protobuf-dev

# Install Go protobuf plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Generate protobuf files
RUN mkdir -p internal/proto && \
    protoc --go_out=internal --go_opt=paths=source_relative \
           --go-grpc_out=internal --go-grpc_opt=paths=source_relative \
           proto/flowguard.proto

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o flowguard ./cmd/flowguard

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/flowguard .

# Expose ports
EXPOSE 8080 9090 9091 9092

# Set environment variables with defaults
ENV UPSTREAM_URL=https://api.openai.com
ENV PROXY_PORT=8080
ENV METRICS_PORT=9090
ENV CONFIG_PORT=9091
ENV GRPC_PORT=9092

# Run the binary
CMD ["./flowguard"] 