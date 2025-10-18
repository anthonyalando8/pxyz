# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum from notification-service
COPY services/common-services/comms-services/notification-service/go.mod services/common-services/comms-services/notification-service/go.sum ./

# Copy shared package so deps are resolvable
COPY shared /shared

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download
RUN go mod tidy

# Copy secrets (if needed at build time)
COPY services/common-services/comms-services/notification-service/secrets ./secrets

# Copy notification-service source
COPY services/common-services/comms-services/notification-service/ ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o notification-service ./cmd/main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/notification-service /app/notification-service

# Copy templates for runtime usage
COPY services/common-services/comms-services/notification-service/templates ./templates

RUN mkdir -p /app/uploads

RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot

EXPOSE 8013 8014

ENTRYPOINT ["/app/notification-service"]

