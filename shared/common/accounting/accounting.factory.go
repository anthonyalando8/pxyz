package accountingclient

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	accountingpb "x/shared/genproto/shared/accounting/accountingpb" // <-- adjust path to your proto

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AccountingClient struct {
	Client accountingpb.AccountingServiceClient
}

// NewAccountingClient connects to the accounting service and returns a ready-to-use client
func NewAccountingClient() *AccountingClient {
	// Default to accounting-service:8024 if env var not set
	accountingServiceAddr := getEnv("ACCOUNTING_SERVICE_ADDR", "accounting-service:8024")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, accountingServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to accounting service: %v", err)
	}

	fmt.Println("Connected to Accounting Service at", accountingServiceAddr)

	client := accountingpb.NewAccountingServiceClient(conn)
	return &AccountingClient{Client: client}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
