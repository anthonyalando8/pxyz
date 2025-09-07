package coreclient

import (
	"log"
	"os"

	corepb "x/shared/genproto/corepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CoreService struct {
	Client corepb.CoreServiceClient
	Conn   *grpc.ClientConn
}

func NewCoreService() *CoreService {
	coreAddr := getEnv("CORE_SERVICE_ADDR", "core-service:8052")

	conn, err := grpc.Dial(coreAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Core service at %s: %v", coreAddr, err)
	}

	client := corepb.NewCoreServiceClient(conn)
	return &CoreService{
		Client: client,
		Conn:   conn,
	}
}

func (c *CoreService) Close() error {
	return c.Conn.Close()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
