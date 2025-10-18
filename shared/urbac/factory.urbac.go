package rbacclient

import (
	"log"
	"os"

	rbacpb "x/shared/genproto/urbacpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RBACService struct {
	Client rbacpb.RBACServiceClient
	Conn   *grpc.ClientConn
}

func NewRBACService() *RBACService {
	rbacAddr := getEnv("RBAC_SERVICE_ADDR", "u-access-service:8032")

	conn, err := grpc.Dial(rbacAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to RBAC service at %s: %v", rbacAddr, err)
	}

	client := rbacpb.NewRBACServiceClient(conn)
	return &RBACService{
		Client: client,
		Conn:   conn,
	}
}

func (r *RBACService) Close() error {
	return r.Conn.Close()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
