# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

WORKDIR /app

# Copy go.mod and go.sum for sms-service
COPY services/sms-service/go.mod services/sms-service/go.sum ./

# Copy shared package so deps are resolvable
COPY shared /shared

RUN go mod download
RUN go mod tidy

# Copy sms-service source
COPY services/sms-service/ ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o sms-service ./cmd/sms.main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app

COPY --from=builder /app/sms-service /app/sms-service

RUN useradd -u 10001 nonroot && chown -R nonroot:nonroot /app
USER nonroot

EXPOSE 8012

ENTRYPOINT ["/app/sms-service"]
