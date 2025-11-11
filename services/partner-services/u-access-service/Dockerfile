# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum from u-access-service
COPY services/partner-services/u-access-service/go.mod services/partner-services/u-access-service/go.sum ./ 

# Copy shared package so deps are resolvable
COPY shared /shared

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download
RUN go mod tidy

# Copy secrets (if needed at build time)
COPY services/partner-services/u-access-service/secrets ./secrets

# Copy u-access-service source
COPY services/partner-services/u-access-service/ ./ 

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o u-access-service ./cmd/u_access.main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd \
 && rm -rf /var/lib/apt/lists/*

# Copy only the binary from builder stage
COPY --from=builder /app/u-access-service /app/u-access-service

# Create uploads directory (if needed for RBAC service)
RUN mkdir -p /app/uploads

RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot

# Expose REST & gRPC ports (customize if RBAC uses diff ports)
EXPOSE 7504 7505

# Run the service
ENTRYPOINT ["/app/ptn-access-service"]
