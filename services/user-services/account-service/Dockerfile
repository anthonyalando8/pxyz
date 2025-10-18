FROM golang:1.24 AS builder
WORKDIR /app
COPY services/account-service/go.mod services/account-service/go.sum ./
COPY shared /shared
RUN go mod download
# Ensure modules are tidy
RUN go mod tidy
COPY services/account-service/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o account-service ./cmd/acc.main.go

FROM debian:bookworm-slim
COPY --from=builder /app/account-service /account-service
USER nonroot:nonroot
EXPOSE 8004
ENTRYPOINT ["/account-service"]
