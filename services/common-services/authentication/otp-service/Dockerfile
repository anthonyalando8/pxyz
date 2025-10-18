FROM golang:1.24 AS builder
WORKDIR /app
COPY services/common-services/authentication/otp-service/go.mod services/common-services/authentication/otp-service/go.sum ./
COPY shared /shared
RUN go mod download
RUN go mod tidy
COPY services/common-services/authentication/otp-service/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o otp-service ./cmd/otp.main.go

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /app/otp-service /otp-service

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd \
 && rm -rf /var/lib/apt/lists/*

RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot
EXPOSE 8003
ENTRYPOINT ["/otp-service"]
