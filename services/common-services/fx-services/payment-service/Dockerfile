# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

WORKDIR /app

COPY services/common-services/fx-services/payment-service/go.mod services/common-services/fx-services/payment-service/go.sum ./
COPY shared /shared

RUN go mod download
RUN go mod tidy

COPY services/common-services/fx-services/payment-service/ ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o payment-service ./cmd/server/main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app

#  Install curl for healthcheck
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/payment-service /app/payment-service

# Copy M-Pesa certificate
COPY services/common-services/fx-services/payment-service/certs /app/certs

RUN mkdir -p /app/uploads

# Non-root user for security
RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app


USER nonroot

EXPOSE 8027

ENTRYPOINT ["/app/payment-service"]