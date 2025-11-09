# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum from partner-service
COPY services/partner-services/partner-service/go.mod services/partner-services/partner-service/go.sum ./

# Copy shared packages
COPY shared /shared

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download
RUN go mod tidy

# Copy secrets if needed at build time
COPY services/partner-services/partner-service/secrets ./secrets

# Copy partner-service source
COPY services/partner-services/partner-service/ ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o partner-service ./cmd/ptn.main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd \
 && rm -rf /var/lib/apt/lists/*

# Copy binary from builder stage
COPY --from=builder /app/partner-service /app/partner-service

# Create uploads directory for partner-related files
RUN mkdir -p /app/uploads

# Non-root user for security
RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot

# Expose REST port
EXPOSE 7510

# Run the service
ENTRYPOINT ["/app/partner-service"]
