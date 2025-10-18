package receiptclient

import (
	"context"
	"fmt"
	"log"
	"time"

	receiptpb "x/shared/genproto/shared/accounting/receipt/v2" // adjust path to your v2 proto

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ReceiptClientV2 struct {
	Client receiptpb.ReceiptServiceV2Client
	conn   *grpc.ClientConn
}

// NewReceiptClientV2 connects to the receipt service and returns a ready-to-use v2 client
func NewReceiptClientV2() *ReceiptClientV2 {
	// Default to receipt-service:8026 if env var not set
	receiptServiceAddr := getEnv("RECEIPT_SERVICE_ADDR", "receipt-service:8026")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, receiptServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to receipt service (v2): %v", err)
	}

	fmt.Println("Connected to Receipt Service V2 at", receiptServiceAddr)

	client := receiptpb.NewReceiptServiceV2Client(conn)
	return &ReceiptClientV2{Client: client, conn: conn}
}

// Close releases the gRPC connection
func (c *ReceiptClientV2) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}