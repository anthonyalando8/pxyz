# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum from repo root
COPY partner-services/ptn-session-mngt/go.mod partner-services/ptn-session-mngt/go.sum ./

# Copy shared package so deps are resolvable
COPY shared /shared

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download
RUN go mod tidy
# Copy secrets (if needed at build time)
COPY partner-services/ptn-session-mngt/secrets ./secrets
# Copy ptn-session-mngt service source
COPY partner-services/ptn-session-mngt/ .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o ptn-session-service ./cmd/sess.main.go

# -------- Stage 2: Minimal Runtime --------
FROM debian:bookworm-slim

# Copy only the binary from builder stage
COPY --from=builder /app/ptn-session-service /ptn-session-service

# Use non-root user for security
USER nonroot:nonroot

# Expose gRPC port
EXPOSE 7503

# Run the service
ENTRYPOINT ["/ptn-session-service"]
