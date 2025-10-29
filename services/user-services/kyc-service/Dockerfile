# -------- Stage 1: Builder -------- 
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum from kyc-service
COPY services/user-services/kyc-service/go.mod services/user-services/kyc-service/go.sum ./

# Copy shared package so deps are resolvable
COPY shared /shared

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download

# Ensure modules are tidy
RUN go mod tidy

# Copy secrets (if needed at build time)
COPY services/user-services/kyc-service/secrets ./secrets

# Copy kyc-service source
COPY services/user-services/kyc-service/ ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o kyc-service ./cmd/kyc.main.go

# -------- Stage 2: Runtime --------
FROM debian:bookworm-slim

WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates passwd \
 && rm -rf /var/lib/apt/lists/*
# Copy only the binary from builder stage
COPY --from=builder /app/kyc-service /app/kyc-service

# Create uploads directory for KYC documents
# RUN mkdir -p /app/uploads/kyc_docs

# Non-root user for security
RUN set -eux; \
    if ! id -u nonroot >/dev/null 2>&1; then \
        adduser --system --uid 10001 --group --no-create-home nonroot; \
    fi; \
    chown -R nonroot:nonroot /app

USER nonroot

RUN mkdir -p /app/uploads/kyc_docs \
 && chown -R nonroot:nonroot /app \
 && chmod -R 775 /app/uploads


# Expose gRPC/REST port
EXPOSE 8005

# Run the service
ENTRYPOINT ["/app/kyc-service"]
