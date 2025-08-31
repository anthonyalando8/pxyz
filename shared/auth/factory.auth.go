// pkg/authclient/auth_client.go
package authclient

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	authpb "x/shared/genproto/authpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AuthService struct {
	Client authpb.AuthServiceClient
}

// NewAuthService connects to the Auth microservice and returns a ready-to-use client wrapper.
func NewAuthService() *AuthService {
	authAddr := getEnv("AUTH_SERVICE_ADDR", "auth-service:8006")

	if wd, err := os.Getwd(); err == nil {
		fmt.Println("Working directory:", wd)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Auth service at %s: %v", authAddr, err)
	}

	client := authpb.NewAuthServiceClient(conn)
	return &AuthService{
		Client: client,
	}
}

// Helper to read env vars with fallback
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Convenience methods for commonly used RPCs
func (a *AuthService) RegisterUser(ctx context.Context, req *authpb.RegisterUserRequest) (*authpb.RegisterUserResponse, error) {
	return a.Client.RegisterUser(ctx, req)
}

func (a *AuthService) GetUserProfile(ctx context.Context, req *authpb.GetUserProfileRequest) (*authpb.GetUserProfileResponse, error) {
	return a.Client.GetUserProfile(ctx, req)
}

func (a *AuthService) GetUserRolesPermissions(ctx context.Context, req *authpb.GetUserRolesPermissionsRequest) (*authpb.GetUserRolesPermissionsResponse, error) {
	return a.Client.GetUserRolesPermissions(ctx, req)
}
