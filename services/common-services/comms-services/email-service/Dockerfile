# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

WORKDIR /app

# Copy go.mod and go.sum
COPY services/common-services/comms-services/email-service/go.mod services/common-services/comms-services/email-service/go.sum ./

# Copy shared dependencies
COPY shared /shared

# Download deps
RUN go mod download

# Copy email-service source
COPY services/common-services/comms-services/email-service/ .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o email-service ./cmd/email.main.go

# -------- Stage 2: Minimal Runtime --------
FROM debian:bookworm-slim
WORKDIR /app
RUN useradd -u 65532 nonroot
COPY --from=builder /app/email-service /email-service

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd \
 && rm -rf /var/lib/apt/lists/*

 RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot
EXPOSE 8011
ENTRYPOINT ["/email-service"]