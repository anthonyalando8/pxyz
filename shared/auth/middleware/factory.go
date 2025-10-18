package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"x/shared/auth/pkg/jwtutil"
	adminsessionpb "x/shared/genproto/admin/sessionpb"
	ptnsessionpb "x/shared/genproto/partner/sessionpb"
	authpb "x/shared/genproto/sessionpb"
	"x/shared/genproto/urbacpb"
	rbacclient "x/shared/urbac"
	"x/shared/utils/cache"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type MiddlewareWithClient struct {
	Client        authpb.AuthServiceClient
	PartnerClient ptnsessionpb.PartnerSessionServiceClient
	AdminClient   adminsessionpb.AdminSessionServiceClient
	RBACClient    urbacpb.RBACServiceClient

	Middleware func(http.Handler) http.Handler
	// Require now takes session types, purposes, and roles
	Require   func(allowedTypes, allowedPurposes, allowedRoles []string) func(http.Handler) http.Handler
	RateLimit func(rdb *redis.Client, limit int, window time.Duration, blockDuration time.Duration, keyPrefix string) func(http.Handler) http.Handler
}

// RequireAuth initializes middleware + all clients
func RequireAuth() *MiddlewareWithClient {
	if wd, err := os.Getwd(); err == nil {
		fmt.Println("Working directory:", wd)
	}

	pubPath := getEnv("JWT_PUBLIC_KEY_PATH", "../../shared/secrets/jwt_public.pem")
	pub, err := jwtutil.LoadRSAPublicKeyFromPEM(pubPath)
	if err != nil || pub == nil {
		log.Fatalf("failed to load public key from %s: %v", pubPath, err)
	}

	// Initialize separate verifiers for user, partner, admin
	userVerifier := jwtutil.NewVerifier(pub, getEnv("JWT_USER_ISSUER", "auth-service"), getEnv("JWT_USER_AUDIENCE", "pxyz-clients"))
	partnerVerifier := jwtutil.NewVerifier(pub, getEnv("JWT_PARTNER_ISSUER", "ptn-auth-service"), getEnv("JWT_PARTNER_AUDIENCE", "pxyz-ptn-clients"))
	adminVerifier := jwtutil.NewVerifier(pub, getEnv("JWT_ADMIN_ISSUER", "admin-auth-service"), getEnv("JWT_ADMIN_AUDIENCE", "pxyz-admin-clients"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect to main session/auth service
	connAuth, err := grpc.DialContext(ctx, "session-service:8002", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to session-service: %v", err)
	}
	authClient := authpb.NewAuthServiceClient(connAuth)

	// Connect to partner session service
	connPartner, err := grpc.DialContext(ctx, "ptn-session-service:7503", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to ptn-session-service: %v", err)
	}
	partnerClient := ptnsessionpb.NewPartnerSessionServiceClient(connPartner)

	// Connect to admin session service
	connAdmin, err := grpc.DialContext(ctx, "admin-session-service:7003", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to admin-session-service: %v", err)
	}
	adminClient := adminsessionpb.NewAdminSessionServiceClient(connAdmin)

	// Connect to RBAC service
	rbacService := rbacclient.NewRBACService() // uses default or RBAC_SERVICE_ADDR env
	cache := cache.NewCache([]string{"redis:6379"}, "", false)

	// Initialize AuthMiddleware with all verifiers and clients
	m := NewAuthMiddleware(
		userVerifier,
		partnerVerifier,
		adminVerifier,
		authClient,
		partnerClient,
		adminClient,
		rbacService.Client,
		cache,
	)

	return &MiddlewareWithClient{
		Middleware:    m.AuthMiddleware,
		Client:        authClient,
		PartnerClient: partnerClient,
		AdminClient:   adminClient,
		Require:       m.RequireWithChecks, // now supports types, purposes, and roles
		RateLimit:     RateLimiter,
		RBACClient:    rbacService.Client,
	}
}

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
