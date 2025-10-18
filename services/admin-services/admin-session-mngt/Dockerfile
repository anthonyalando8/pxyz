# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum from repo root
COPY admin-services/admin-session-mngt/go.mod admin-services/admin-session-mngt/go.sum ./

# Copy shared package so deps are resolvable
COPY shared /shared

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download
RUN go mod tidy
# Copy secrets (if needed at build time)
COPY admin-services/admin-session-mngt/secrets ./secrets
# Copy admin-session-mngt service source
COPY admin-services/admin-session-mngt/ .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o admin-session-service ./cmd/sess.main.go

# -------- Stage 2: Minimal Runtime --------
FROM debian:bookworm-slim

# Copy only the binary from builder stage
COPY --from=builder /app/admin-session-service /admin-session-service

# Use non-root user for security
USER nonroot:nonroot

# Expose gRPC port
EXPOSE 7003

# Run the service
ENTRYPOINT ["/admin-session-service"]
