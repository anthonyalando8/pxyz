package smsclient

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	smswhatsapppb "x/shared/genproto/smswhatsapppb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type SMSClient struct {
	client smswhatsapppb.SMSWhatsAppServiceClient
}

// NewSMSClient connects to the SMS service and returns a ready-to-use client
func NewSMSClient() *SMSClient {
	smsServiceAddr := getEnv("SMS_SERVICE_ADDR", "sms-service:8012")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, smsServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to SMS service: %v", err)
	}

	fmt.Println("Connected to SMS Service at", smsServiceAddr)

	client := smswhatsapppb.NewSMSWhatsAppServiceClient(conn)
	return &SMSClient{client: client}
}

// SendMessage sends an SMS/WhatsApp message via the gRPC service
func (sc *SMSClient) SendMessage(ctx context.Context, req *smswhatsapppb.SendMessageRequest) (*smswhatsapppb.SendMessageResponse, error) {
	return sc.client.SendMessage(ctx, req)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
