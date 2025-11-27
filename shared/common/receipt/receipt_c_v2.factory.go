package receiptclient

import (
	"context"
	"fmt"
	"log"
	"syscall"
	"time"

	receiptpb "x/shared/genproto/shared/accounting/receipt/v3" // adjust path to your v2 proto

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ReceiptClientV3 struct {
	Client receiptpb.ReceiptServiceClient
	conn   *grpc.ClientConn
}

// NewReceiptClientV2 connects to the receipt service and returns a ready-to-use v2 client
func NewReceiptClientV3() *ReceiptClientV3 {
	// Default to receipt-service:8026 if env var not set
	receiptServiceAddr := getEnv("RECEIPT_SERVICE_ADDR", "receipt-service:8026")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, receiptServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to receipt service (v3): %v", err)
	}

	fmt.Println("Connected to Receipt Service V3 at", receiptServiceAddr)

	client := receiptpb.NewReceiptServiceClient(conn)
	return &ReceiptClientV3{Client: client, conn: conn}
}

// Close releases the gRPC connection
func (c *ReceiptClientV3) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func getEnv(key, defaultVal string) string {
	if value, exists := syscall.Getenv(key); exists {
		return value
	}
	return defaultVal
}
