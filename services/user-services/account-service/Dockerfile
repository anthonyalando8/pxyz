FROM golang:1.24 AS builder
WORKDIR /app
COPY services/user-services/account-service/go.mod services/user-services/account-service/go.sum ./
COPY shared /shared
RUN go mod download
# Ensure modules are tidy
RUN go mod tidy
COPY services/user-services/account-service/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o account-service ./cmd/acc.main.go

FROM debian:bookworm-slim
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd \
 && rm -rf /var/lib/apt/lists/*

# Copy only the binary from builder stage
COPY --from=builder /app/auth-service /app/auth-service

# Create uploads directory for profile pictures
RUN mkdir -p /app/uploads/profile_pictures

# Non-root user for security
RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot
EXPOSE 8004
ENTRYPOINT ["/account-service"]
