FROM golang:1.24 AS builder
WORKDIR /app
COPY services/otp-service/go.mod services/otp-service/go.sum ./
COPY shared /shared
RUN go mod download
RUN go mod tidy
COPY services/otp-service/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o otp-service ./cmd/otp.main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/otp-service /otp-service
USER nonroot:nonroot
EXPOSE 50055
ENTRYPOINT ["/otp-service"]
