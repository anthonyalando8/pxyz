// pkg/otpclient/otp_client.go
package otpclient

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	otppb "x/shared/genproto/otppb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type OTPService struct {
	Client otppb.OTPServiceClient
}

// NewOTPService connects to the OTP microservice and returns a ready-to-use client wrapper.
func NewOTPService() *OTPService {
	otpAddr := getEnv("OTP_SERVICE_ADDR", "otp-service:8003")

	if wd, err := os.Getwd(); err == nil {
		fmt.Println("Working directory:", wd)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, otpAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to OTP service at %s: %v", otpAddr, err)
	}

	client := otppb.NewOTPServiceClient(conn)
	return &OTPService{
		Client: client,
	}
}

// Helper to read env vars with fallback
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
