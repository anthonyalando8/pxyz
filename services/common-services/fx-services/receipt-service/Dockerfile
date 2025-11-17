# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum for receipt-service
COPY services/common-services/receipt-service/go.mod services/common-services/receipt-service/go.sum ./

# Copy shared package for dependency resolution
COPY shared /shared

# Download dependencies
RUN go mod download
RUN go mod tidy

# Copy receipt-service source
COPY services/common-services/receipt-service/ ./ 

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o receipt-service ./cmd/main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd \
 && rm -rf /var/lib/apt/lists/*
# Copy the binary from builder
COPY --from=builder /app/receipt-service /app/receipt-service

# Create uploads directory (optional)
RUN mkdir -p /app/uploads

RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot

# Expose gRPC port for receipt-service
EXPOSE 8026

# Run the service
ENTRYPOINT ["/app/receipt-service"]
