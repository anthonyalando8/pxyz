# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum for payment-service
COPY services/common-services/fx-services/payment-service/go.mod services/common-services/fx-services/payment-service/go.sum ./

# Copy shared package for dependency resolution
COPY shared /shared

# Download dependencies
RUN go mod download
RUN go mod tidy

# Copy payment-service source
COPY services/common-services/fx-services/payment-service/ ./ 

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o payment-service ./cmd/server/main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd \
 && rm -rf /var/lib/apt/lists/*

# Copy the binary from builder
COPY --from=builder /app/payment-service /app/payment-service

# Create uploads directory (optional)
RUN mkdir -p /app/uploads

# Non-root user for security
RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot

# Expose gRPC port for payment-service
EXPOSE 8027

# Run the service
ENTRYPOINT ["/app/payment-service"]
