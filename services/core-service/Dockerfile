# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum from core-service
COPY services/core-service/go.mod services/core-service/go.sum ./

# Copy shared package so deps are resolvable
COPY shared /shared

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download
RUN go mod tidy

# Copy secrets (if needed at build time)
COPY services/core-service/secrets ./secrets

# Copy core-service source
COPY services/core-service/ ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o core-service ./cmd/core.main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app

# Install CA certificates for HTTPS
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Copy only the binary from builder stage
COPY --from=builder /app/core-service /app/core-service

# Create uploads directory (if needed for future services)
RUN mkdir -p /app/uploads

# Non-root user for security
RUN useradd -u 10001 nonroot && chown -R nonroot:nonroot /app
USER nonroot

# Expose REST & gRPC ports
EXPOSE 8051 8052

# Run the service
ENTRYPOINT ["/app/core-service"]
