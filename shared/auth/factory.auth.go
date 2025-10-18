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
	AllAuthServices    ServiceType = "all"
)

type AuthService struct {
	UserClient    authpb.AuthServiceClient
	PartnerClient ptnauthpb.PartnerAuthServiceClient
	AdminClient   adminauthpb.AdminAuthServiceClient
	conns         []*grpc.ClientConn // keep multiple conns if needed
}

// DialAuthService connects to one or all services based on service type
func DialAuthService(service ServiceType) (*AuthService, error) {
	as := &AuthService{}

	switch service {
	case UserAuthService, PartnerAuthService, AdminAuthService:
		// existing single-service logic
		client, conn, err := dialOne(service)
		if err != nil {
			return nil, err
		}
		as.conns = append(as.conns, conn)
		as.UserClient = client.UserClient
		as.PartnerClient = client.PartnerClient
		as.AdminClient = client.AdminClient

	case AllAuthServices:
		// dial all three
		for _, s := range []ServiceType{UserAuthService, PartnerAuthService, AdminAuthService} {
			client, conn, err := dialOne(s)
			if err != nil {
				return nil, fmt.Errorf("failed to dial %s: %w", s, err)
			}
			as.conns = append(as.conns, conn)

			if client.UserClient != nil {
				as.UserClient = client.UserClient
			}
			if client.PartnerClient != nil {
				as.PartnerClient = client.PartnerClient
			}
			if client.AdminClient != nil {
				as.AdminClient = client.AdminClient
			}
		}
	default:
		return nil, fmt.Errorf("unknown service type: %s", service)
	}

	return as, nil
}

// helper to dial one service
func dialOne(service ServiceType) (*AuthService, *grpc.ClientConn, error) {
	var addr string
	switch service {
	case UserAuthService:
		addr = "auth-service:8006"
	case PartnerAuthService:
		addr = "ptn-auth-service:7501"
	case AdminAuthService:
		addr = "admin-auth-service:7001"
	default:
		return nil, nil, fmt.Errorf("unknown service type: %s", service)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Printf("[INFO] Connecting to %s at %s...", service, addr)
	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}

	as := &AuthService{}
	switch service {
	case UserAuthService:
		as.UserClient = authpb.NewAuthServiceClient(conn)
	case PartnerAuthService:
		as.PartnerClient = ptnauthpb.NewPartnerAuthServiceClient(conn)
	case AdminAuthService:
		as.AdminClient = adminauthpb.NewAdminAuthServiceClient(conn)
	}
	return as, conn, nil
}


// Close the connection
func (s *AuthService) Close() error {
	var firstErr error
	for _, conn := range s.conns {
		if err := conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}




// Helper to read env vars with fallback
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
