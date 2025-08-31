package accountclient

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	accountpb "x/shared/genproto/accountpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AccountClient struct {
	Client accountpb.AccountServiceClient
}

// NewAccountClient connects to the email service and returns a ready-to-use client
func NewAccountClient() *AccountClient {
	accServiceAddr := getEnv("ACCOUNT_SERVICE_ADDR", "account-service:8004")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, accServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to account service: %v", err)
	}

	fmt.Println("Connected to account Service at", accServiceAddr)

	client := accountpb.NewAccountServiceClient(conn)
	return &AccountClient{Client: client}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
