# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

WORKDIR /app

# Copy go.mod and go.sum for sms-service
COPY services/common-services/comms-services/sms-service/go.mod services/common-services/comms-services/sms-service/go.sum ./

# Copy shared package so deps are resolvable
COPY shared /shared

RUN go mod download
RUN go mod tidy

# Copy sms-service source
COPY services/common-services/comms-services/sms-service/ ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o sms-service ./cmd/sms.main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/sms-service /app/sms-service

# Create nonroot user + group, then assign ownership
# RUN set -eux; \
#     adduser --system --uid 10001 --group --no-create-home nonroot; \
#     chown -R nonroot:nonroot /app
RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot

EXPOSE 8012

ENTRYPOINT ["/app/sms-service"]
