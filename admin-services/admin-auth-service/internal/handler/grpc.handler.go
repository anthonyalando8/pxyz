package handler

import (
	"admin-auth-service/internal/domain"
	"admin-auth-service/internal/usecase"
	//authclient "x/shared/auth"
	otpclient "x/shared/auth/otp"
	emailclient "x/shared/email"
	authpb "x/shared/genproto/admin/authpb"
	smsclient "x/shared/sms"

	"context"
	"fmt"
	"log"
)

type GRPCAuthHandler struct {
	authpb.UnimplementedAdminAuthServiceServer // embed for forward compat

	uc             *usecase.UserUsecase
	otp            *otpclient.OTPService
	emailClient    *emailclient.EmailClient
	smsClient      *smsclient.SMSClient
	//authClient     *authclient.AuthService
}

func NewGRPCAuthHandler(
	uc *usecase.UserUsecase,
	otp *otpclient.OTPService,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
	//authClient *authclient.AuthService,

) *GRPCAuthHandler {
	return &GRPCAuthHandler{
		uc:             uc,
		otp:            otp,
		emailClient:    emailClient,
		smsClient:      smsClient,
		//authClient:     authClient,
	}
}

//
// ---- gRPC methods ----
//

// Register a new user
func (h *GRPCAuthHandler) RegisterUser(ctx context.Context, req *authpb.RegisterUserRequest) (*authpb.RegisterUserResponse, error) {
	//check if valid role
	user, err := h.uc.RegisterUser(ctx, req.Email, req.Password, req.FirstName, req.LastName, req.Role)
	_ = user
	if err != nil {
		log.Printf("[ERROR] RegisterUser failed: %v", err)
		return &authpb.RegisterUserResponse{
			Ok:    false,
			Error: err.Error(),
		}, nil
	}

	return &authpb.RegisterUserResponse{
		Ok: true,
		UserId: user.ID,
	}, nil
}

// DeleteUser removes a user by ID
func (h *GRPCAuthHandler) DeleteUser(ctx context.Context, req *authpb.DeleteUserRequest) (*authpb.DeleteUserResponse, error) {
	if req.UserId == "" {
		return &authpb.DeleteUserResponse{
			Ok:    false,
			Error: "user_id is required",
		}, nil
	}

	err := h.uc.DeleteUser(ctx, req.UserId)
	if err != nil {
		log.Printf("[ERROR] DeleteUser failed for user_id=%s: %v", req.UserId, err)
		return &authpb.DeleteUserResponse{
			Ok:    false,
			Error: err.Error(),
		}, nil
	}

	return &authpb.DeleteUserResponse{
		Ok: true,
	}, nil
}


// Get full profile (auth + account-service)
func (h *GRPCAuthHandler) GetUserProfile(ctx context.Context, req *authpb.GetUserProfileRequest) (*authpb.GetUserProfileResponse, error) {
	_, err := h.GetFullUserProfile(ctx, req.UserId)
	if err != nil {
		log.Printf("[ERROR] GetFullUserProfile failed: %v", err)
	}

	return nil,nil
}

//
// ---- helper ----
//

// Combines user info (admin-auth-service) + profile info (account-service)
func (h *GRPCAuthHandler) GetFullUserProfile(ctx context.Context, userID string) (*domain.User, error) {
	// Get user
	user, err := h.uc.GetProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get profile from account-service
	

	return user, nil
}



