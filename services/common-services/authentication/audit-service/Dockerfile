# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum from repo root
COPY services/common-services/authentication/audit-service/go.mod services/common-services/authentication/audit-service/go.sum ./

# Copy shared package so deps are resolvable
COPY shared /shared

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download
# Ensure modules are tidy
RUN go mod tidy

# Copy secrets (if needed at build time)
COPY services/common-services/authentication/audit-service/secrets ./secrets

# Copy audit-service source
COPY services/common-services/authentication/audit-service/ ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o audit-service ./cmd/audit.main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd \
 && rm -rf /var/lib/apt/lists/*

# Copy only the binary from builder stage
COPY --from=builder /app/audit-service /app/audit-service

# Create uploads directory for profile pictures
RUN mkdir -p /app/uploads/profile_pictures

RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot

# Expose REST port
EXPOSE 8007 8008

# Run the service
ENTRYPOINT ["/app/audit-service"]