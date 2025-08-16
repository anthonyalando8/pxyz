# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum from repo root
COPY services/session-mngt/go.mod services/session-mngt/go.sum ./

# Copy shared package so deps are resolvable
COPY shared /shared

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download
# Copy secrets (if needed at build time)
COPY services/session-mngt/secrets ./secrets
# Copy session-mngt service source
COPY services/session-mngt/ .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o session-service ./cmd/sess.main.go

# -------- Stage 2: Minimal Runtime --------
FROM gcr.io/distroless/static:nonroot

# Copy only the binary from builder stage
COPY --from=builder /app/session-service /session-service

# Use non-root user for security
USER nonroot:nonroot

# Expose gRPC port
EXPOSE 50050

# Run the service
ENTRYPOINT ["/session-service"]
