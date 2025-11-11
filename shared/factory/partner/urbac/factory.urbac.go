package rbacclient

import (
	"log"
	"os"

	rbacpb "x/shared/genproto/partner/ptnrbacpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PartnerRBACService struct {
	Client rbacpb.RBACServiceClient
	Conn   *grpc.ClientConn
}

func NewRBACService() *PartnerRBACService {
	rbacAddr := getEnv("RBAC_SERVICE_ADDR", "ptn-access-service:7505")

	conn, err := grpc.Dial(rbacAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to RBAC service at %s: %v", rbacAddr, err)
	}

	client := rbacpb.NewRBACServiceClient(conn)
	return &PartnerRBACService{
		Client: client,
		Conn:   conn,
	}
}

func (r *PartnerRBACService) Close() error {
	return r.Conn.Close()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
