# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum for p2p-service
COPY services/user-services/p2p-service/go.mod services/user-services/p2p-service/go.sum ./

# Copy shared package so deps are resolvable
COPY shared /shared

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download
RUN go mod tidy

# Copy secrets (if needed at build time)
COPY services/user-services/p2p-service/secrets ./secrets

# Copy p2p-service source
COPY services/user-services/p2p-service/ ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o p2p-service ./cmd/main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd curl \
 && rm -rf /var/lib/apt/lists/*

# Copy only the binary from builder stage
COPY --from=builder /app/p2p-service /app/p2p-service

# Non-root user for security
RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot

# Expose REST/gRPC port
EXPOSE 8030 8031

# Run the service
ENTRYPOINT ["/app/p2p-service"]
