package handler

import (
	"auth-service/internal/domain"
	telegramclient "auth-service/internal/service/telegram"
	"auth-service/internal/usecase"
	"time"

	accountclient "x/shared/account"
	//authclient "x/shared/auth"
	otpclient "x/shared/auth/otp"
	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	notificationclient "x/shared/notification" // ✅ added

	accountpb "x/shared/genproto/accountpb"
	authpb "x/shared/genproto/authpb"

	"context"
	"fmt"
	"log"
)

type GRPCAuthHandler struct {
	authpb.UnimplementedAuthServiceServer // embed for forward compat

	uc                 *usecase.UserUsecase
	otp                *otpclient.OTPService
	accountClient      *accountclient.AccountClient
	emailClient        *emailclient.EmailClient
	smsClient          *smsclient.SMSClient
	notificationClient *notificationclient.NotificationService // ✅ added
	//authClient       *authclient.AuthService
	config         *Config
	telegramClient *telegramclient.TelegramClient
}

func NewGRPCAuthHandler(
	uc *usecase.UserUsecase,
	otp *otpclient.OTPService,
	accountClient *accountclient.AccountClient,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
	notificationClient *notificationclient.NotificationService, // ✅ added
	//authClient *authclient.AuthService,
	config *Config,
	telegramClient *telegramclient.TelegramClient,
) *GRPCAuthHandler {
	return &GRPCAuthHandler{
		uc:                 uc,
		otp:                otp,
		accountClient:      accountClient,
		emailClient:        emailClient,
		smsClient:          smsClient,
		notificationClient: notificationClient, // ✅ assigned
		//authClient:       authClient,
		config:         config,
		telegramClient: telegramClient,
	}
}


//
// ---- gRPC methods ----
//

// Register a new user
func (h *GRPCAuthHandler) RegisterUser(ctx context.Context, req *authpb.RegisterUserRequest) (*authpb.RegisterUserResponse, error) {
	//check if valid role
	if !domain.IsValidRole(req.Role) {
		log.Printf("[ERROR] RegisterUser failed: invalid role %v", req.Role)
		return &authpb.RegisterUserResponse{
			Ok:    false,
			Error: "invalid role",
		}, nil
	}
	request := &usecase.RegisterUserRequest{
		Email:     &req.Email,
		Password:  &req.Password,
		Consent:   false,
		IsEmailVerified: true,
		AccountType: "password", // since password is provided
	}
	user, err := h.uc.RegisterUserAsyncBulk(ctx, []usecase.RegisterUserRequest{*request})
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
		UserId: user[0],
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
	user, profile, err := h.GetFullUserProfile(ctx, req.UserId)
	if err != nil {
		log.Printf("[ERROR] GetFullUserProfile failed: %v", err)
		return &authpb.GetUserProfileResponse{
			Ok:    false,
			Error: err.Error(),
		}, nil
	}

	// Build the proto User object
	pbUser := &authpb.User{
		Id:               user.ID,
		Email:            safeString(user.Email),
		Phone:            safeString(user.Phone),
		IsEmailVerified:  user.IsEmailVerified,
		IsPhoneVerified:  user.IsPhoneVerified,
		AccountType:      user.AccountType,
		AccountStatus:    user.AccountStatus,
		CreatedAt:        user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        user.UpdatedAt.Format(time.RFC3339),

		FirstName:        profile.FirstName,
		LastName:         profile.LastName,
		Bio:              profile.Bio,
		Gender:           profile.Gender,
		DateOfBirth:      profile.DateOfBirth,
		ProfileImage:     profile.ProfileImageUrl,
		AddressJson:      profile.AddressJson, // keep JSON string as-is
	}

	return &authpb.GetUserProfileResponse{
		Ok:   true,
		User: pbUser,
	}, nil
}

//
// ---- helper ----
//

// Combines user info (auth-service) + profile info (account-service)
func (h *GRPCAuthHandler) GetFullUserProfile(ctx context.Context, userID string) (*domain.UserProfile, *accountpb.UserProfile, error) {
	// Get user
	user, err := h.uc.GetProfile(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get profile from account-service
	profileResp, err := h.accountClient.Client.GetUserProfile(ctx, &accountpb.GetUserProfileRequest{
		UserId: userID,
	})
	if err != nil || profileResp == nil || profileResp.Profile == nil {
		return nil, nil, fmt.Errorf("failed to get profile from account-service: %w", err)
	}

	return user, profileResp.Profile, nil
}
