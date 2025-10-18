# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum from repo root
COPY services/common-services/authentication/session-mngt/go.mod services/common-services/authentication/session-mngt/go.sum ./

# Copy shared package so deps are resolvable
COPY shared /shared

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download
RUN go mod tidy
# Copy secrets (if needed at build time)
COPY services/common-services/authentication/session-mngt/secrets ./secrets
# Copy session-mngt service source
COPY services/common-services/authentication/session-mngt/ .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o session-service ./cmd/sess.main.go

# -------- Stage 2: Minimal Runtime --------
FROM debian:bookworm-slim
WORKDIR /app

# Copy only the binary from builder stage
COPY --from=builder /app/session-service /session-service

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd \
 && rm -rf /var/lib/apt/lists/*

 RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot

# Expose gRPC port
EXPOSE 8002

# Run the service
ENTRYPOINT ["/session-service"]
