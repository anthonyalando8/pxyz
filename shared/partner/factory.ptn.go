package partnerclient

import (
	"log"
	"os"

	partnerpb "x/shared/genproto/partner/svcpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PartnerService struct {
	Client partnerpb.PartnerServiceClient
	Conn   *grpc.ClientConn
}

func NewPartnerService() *PartnerService {
	// Default to partner-service:7510 unless overridden by env
	partnerAddr := getEnv("PARTNER_SERVICE_ADDR", "partner-service:7511")

	conn, err := grpc.Dial(partnerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Partner service at %s: %v", partnerAddr, err)
	}

	client := partnerpb.NewPartnerServiceClient(conn)
	return &PartnerService{
		Client: client,
		Conn:   conn,
	}
}

func (p *PartnerService) Close() error {
	return p.Conn.Close()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
