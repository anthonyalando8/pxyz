# -------- Stage 1: Builder --------
FROM golang:1.24 AS builder

WORKDIR /app

# Copy go.mod and go.sum
COPY services/email-service/go.mod services/email-service/go.sum ./

# Copy shared dependencies
COPY shared /shared

# Download deps
RUN go mod download

# Copy email-service source
COPY services/email-service/ .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o email-service ./cmd/email.main.go

# -------- Stage 2: Minimal Runtime --------
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /app/email-service /email-service

USER nonroot:nonroot

EXPOSE 50054

ENTRYPOINT ["/email-service"]
