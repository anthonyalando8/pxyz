// pkg/authclient/auth_client.go
package authclient
import (
	"context"
	"fmt"
	"log"
	"time"
	"os"

	authpb "x/shared/genproto/authpb"
	ptnauthpb "x/shared/genproto/partner/authpb"
	adminauthpb "x/shared/genproto/admin/authpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ServiceType string

const (
	UserAuthService    ServiceType = "user"
	PartnerAuthService ServiceType = "partner"
	AdminAuthService   ServiceType = "admin"
)

type AuthService struct {
	UserClient    authpb.AuthServiceClient
	PartnerClient ptnauthpb.PartnerAuthServiceClient
	AdminClient   adminauthpb.AdminAuthServiceClient
	conn          *grpc.ClientConn
}

// DialAuthService connects to the requested service and returns a wrapper with the appropriate client set
func DialAuthService(service ServiceType) (*AuthService, error) {
	var (
		addr string
		as   = &AuthService{}
	)

	switch service {
	case UserAuthService:
		addr = "auth-service:8006"
	case PartnerAuthService:
		addr = "ptn-auth-service:7501"
	case AdminAuthService:
		addr = "admin-auth-service:7001"
	default:
		return nil, fmt.Errorf("unknown service type: %s", service)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Printf("[INFO] Connecting to %s at %s...", service, addr)
	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", service, err)
	}
	as.conn = conn

	// Set the correct client
	switch service {
	case UserAuthService:
		as.UserClient = authpb.NewAuthServiceClient(conn)
	case PartnerAuthService:
		as.PartnerClient = ptnauthpb.NewPartnerAuthServiceClient(conn)
	case AdminAuthService:
		as.AdminClient = adminauthpb.NewAdminAuthServiceClient(conn)
	}

	return as, nil
}

// Close the connection
func (s *AuthService) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}



// Helper to read env vars with fallback
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
