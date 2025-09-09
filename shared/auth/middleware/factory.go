package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"x/shared/auth/pkg/jwtutil"
	authpb "x/shared/genproto/sessionpb"
	"x/shared/urbac"
	"x/shared/genproto/urbacpb"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type MiddlewareWithClient struct {
	Client       authpb.AuthServiceClient
	RBACClient   urbacpb.RBACServiceClient
	
	Middleware   func(http.Handler) http.Handler
	Require      func(allowedTypes []string, allowedPurposes []string) func(http.Handler) http.Handler
	RequireRole  func(allowedRoles []string) func(http.Handler) http.Handler
	RateLimit    func(rdb *redis.Client, limit int, window time.Duration, blockDuration time.Duration, keyPrefix string) func(http.Handler) http.Handler
}


// RequireAuth initializes middleware + client
func RequireAuth() *MiddlewareWithClient {
	if wd, err := os.Getwd(); err == nil {
		fmt.Println("Working directory:", wd)
	}

	jwtVerifier := jwtutil.LoadAndBuild(jwtutil.JWTConfig{
		PubPath:  getEnv("JWT_PUBLIC_KEY_PATH", "../../shared/secrets/jwt_public.pem"),
		Issuer:   getEnv("JWT_ISSUER", "auth-service"),
		Audience: getEnv("JWT_AUDIENCE", "pxyz-clients"),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect to auth/session service
	connAuth, err := grpc.DialContext(ctx, "session-service:8002", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to auth-service: %v", err)
	}
	authClient := authpb.NewAuthServiceClient(connAuth)

	// Connect to RBAC service
	rbacService := rbacclient.NewRBACService() // uses default or RBAC_SERVICE_ADDR env

	m := NewAuthMiddleware(jwtVerifier, authClient)

	return &MiddlewareWithClient{
		Middleware: m.AuthMiddleware,
		Client:     authClient,
		Require:    m.RequireWithChecks,
		RateLimit:  RateLimiter,
		RequireRole: m.RoleMiddleware,
		// Include RBAC client
		RBACClient: rbacService.Client,
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
