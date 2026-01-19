# -------- Stage 1: Builder --------
FROM golang:1.25 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum for crypto-service
COPY services/common-services/fx-services/crypto-service/go.mod services/common-services/fx-services/crypto-service/go.sum ./

# Copy shared package for dependency resolution
COPY shared /shared

# Download dependencies
RUN go mod download
RUN go mod tidy

# Copy secrets if needed
# COPY services/common-services/fx-services/crypto-service/secrets ./secrets

# Copy crypto-service source
COPY services/common-services/fx-services/crypto-service/ ./ 

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o crypto-service ./cmd/server/main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd \
 && rm -rf /var/lib/apt/lists/*

# Copy the binary from builder
COPY --from=builder /app/crypto-service /app/crypto-service

# Create uploads directory (optional)
RUN mkdir -p /app/uploads

# Non-root user for security
RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot

# Expose gRPC port for crypto-service
EXPOSE 8028

# Run the service
ENTRYPOINT ["/app/crypto-service"]
