# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum for receipt-service
COPY common-services/receipt-service/go.mod common-services/receipt-service/go.sum ./

# Copy shared package for dependency resolution
COPY shared /shared

# Download dependencies
RUN go mod download
RUN go mod tidy

# Copy receipt-service source
COPY common-services/receipt-service/ ./ 

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o receipt-service ./cmd/main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app

# Install CA certificates for HTTPS
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Copy the binary from builder
COPY --from=builder /app/receipt-service /app/receipt-service

# Create uploads directory (optional)
RUN mkdir -p /app/uploads

# Non-root user for security
RUN apt-get update && apt-get install -y --no-install-recommends passwd && \
    rm -rf /var/lib/apt/lists/*

RUN id -u nonroot &>/dev/null || adduser --uid 10001 --disabled-password --gecos "" nonroot && \
    chown -R nonroot:nonroot /app

USER nonroot

# Expose gRPC port for receipt-service
EXPOSE 8026

# Run the service
ENTRYPOINT ["/app/receipt-service"]
