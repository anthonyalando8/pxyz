# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum from repo root
COPY services/common-services/authentication/auth-service/go.mod services/common-services/authentication/auth-service/go.sum ./

# Copy shared package so deps are resolvable
COPY shared /shared

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download
# Ensure modules are tidy
RUN go mod tidy

# Copy secrets (if needed at build time)
COPY services/common-services/authentication/auth-service/secrets ./secrets

# Copy auth-service source
COPY services/common-services/authentication/auth-service/ ./

# Copy UI folder
COPY services/common-services/authentication/auth-service/ui ./ui

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o auth-service ./cmd/auth.main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd \
 && rm -rf /var/lib/apt/lists/*

# Copy only the binary from builder stage
COPY --from=builder /app/auth-service /app/auth-service

# Create uploads directory for profile pictures
RUN mkdir -p /app/uploads/profile_pictures

# Non-root user for security
RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot

# Expose REST port
EXPOSE 8001 8006

# Run the service
ENTRYPOINT ["/app/auth-service"]