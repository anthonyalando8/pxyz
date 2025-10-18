# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy service-specific go.mod and go.sum first (for dependency caching)
COPY services/wallet-service/go.mod services/wallet-service/go.sum ./

# Copy shared package so imports like "x/shared" resolve
COPY shared /shared

# Download dependencies
RUN go mod download

# Copy secrets (if the wallet service needs them at build time)
COPY secrets /app/secrets

# Copy wallet-service source code
COPY services/wallet-service/ .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o wallet-service ./cmd/wlt.main.go

# -------- Stage 2: Minimal Runtime --------
FROM debian:bookworm-slim

# Copy only the binary from builder stage
COPY --from=builder /app/wallet-service /wallet-service

# Use non-root user for security
USER nonroot:nonroot

# Expose gRPC port
EXPOSE 50052

# Start the service
ENTRYPOINT ["/wallet-service"]
