# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum from repo root
COPY admin-services/admin-auth-service/go.mod admin-services/admin-auth-service/go.sum ./

# Copy shared package so deps are resolvable
COPY shared /shared

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download
# Ensure modules are tidy
RUN go mod tidy

# Copy secrets (if needed at build time)
COPY admin-services/admin-auth-service/secrets ./secrets

# Copy admin-auth-service source
COPY admin-services/admin-auth-service/ ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o admin-auth-service ./cmd/auth.main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app

# Install CA certificates for HTTPS
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Copy only the binary from builder stage
COPY --from=builder /app/admin-auth-service /app/admin-auth-service

# Create uploads directory for profile pictures
RUN mkdir -p /app/uploads/profile_pictures

# Non-root user for security
RUN apt-get update && apt-get install -y --no-install-recommends passwd && \
    rm -rf /var/lib/apt/lists/*

RUN id -u nonroot &>/dev/null || adduser --uid 10001 --disabled-password --gecos "" nonroot && \
    chown -R nonroot:nonroot /app

USER nonroot

# Expose REST port
EXPOSE 7000 7001

# Run the service
ENTRYPOINT ["/app/admin-auth-service"]