package emailclient

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	emailpb "x/shared/genproto/emailpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type EmailClient struct {
	client emailpb.EmailServiceClient
}

// NewEmailClient connects to the email service and returns a ready-to-use client
func NewEmailClient() *EmailClient {
	emailServiceAddr := getEnv("EMAIL_SERVICE_ADDR", "email-service:8011")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, emailServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to email service: %v", err)
	}

	fmt.Println("Connected to Email Service at", emailServiceAddr)

	client := emailpb.NewEmailServiceClient(conn)
	return &EmailClient{client: client}
}

// SendEmail sends an email through the gRPC email service
func (ec *EmailClient) SendEmail(ctx context.Context, req *emailpb.SendEmailRequest) (*emailpb.SendEmailResponse, error) {
	return ec.client.SendEmail(ctx, req)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
