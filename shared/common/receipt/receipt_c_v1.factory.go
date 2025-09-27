package receiptclient

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	receiptpb "x/shared/genproto/shared/accounting/receiptpb" // adjust path to your proto

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ReceiptClient struct {
	Client receiptpb.ReceiptServiceClient
}

// NewReceiptClient connects to the receipt service and returns a ready-to-use client
func NewReceiptClient() *ReceiptClient {
	// Default to receipt-service:8026 if env var not set
	receiptServiceAddr := getEnv("RECEIPT_SERVICE_ADDR", "receipt-service:8026")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, receiptServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to receipt service: %v", err)
	}

	fmt.Println("Connected to Receipt Service at", receiptServiceAddr)

	client := receiptpb.NewReceiptServiceClient(conn)
	return &ReceiptClient{Client: client}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
