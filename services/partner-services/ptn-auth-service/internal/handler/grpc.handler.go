package handler

import (
	"context"
	"log"

	"ptn-auth-service/internal/domain"
	"ptn-auth-service/internal/usecase"

	otpclient "x/shared/auth/otp"
	emailclient "x/shared/email"
	authpb "x/shared/genproto/partner/authpb"
	smsclient "x/shared/sms"
)

type GRPCAuthHandler struct {
	authpb.UnimplementedPartnerAuthServiceServer // forward compat

	uc          *usecase.UserUsecase
	otp         *otpclient.OTPService
	emailClient *emailclient.EmailClient
	smsClient   *smsclient.SMSClient
	authHandler  *AuthHandler
}

func NewGRPCAuthHandler(
	uc *usecase.UserUsecase,
	otp *otpclient.OTPService,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
	authHandler  *AuthHandler,
) *GRPCAuthHandler {
	return &GRPCAuthHandler{
		uc:          uc,
		otp:         otp,
		emailClient: emailClient,
		smsClient:   smsClient,
		authHandler:  authHandler,
	}
}

//
// ---------------- Register User ----------------
//
func (h *GRPCAuthHandler) RegisterUser(ctx context.Context, req *authpb.RegisterUserRequest) (*authpb.RegisterUserResponse, error) {
	if req.Email == "" || req.Password == "" {
		return &authpb.RegisterUserResponse{
			Ok:    false,
			Error: "email and password are required",
		}, nil
	}

	// Pass partner_id into usecase so we can associate properly
	user, err := h.uc.RegisterUser(ctx, req.Email, req.Password, req.FirstName, req.LastName, req.Role, req.PartnerId)
	if err != nil {
		log.Printf("[ERROR] RegisterUser failed: %v", err)
		return &authpb.RegisterUserResponse{
			Ok:    false,
			Error: err.Error(),
		}, nil
	}

	err = h.authHandler.HandleRoleUpgrade(ctx, user.ID, req.Role)
	if err != nil {
		log.Printf("failed to assign role %s to user %s: %v", req.Role, user.ID, err)
	}
	
	return &authpb.RegisterUserResponse{
		Ok:     true,
		UserId: user.ID,
	}, nil
}

//
// ---------------- Delete User ----------------
//
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

	return &authpb.DeleteUserResponse{Ok: true}, nil
}

//
// ---------------- Get User Profile ----------------
//
func (h *GRPCAuthHandler) GetUserProfile(ctx context.Context, req *authpb.GetUserProfileRequest) (*authpb.GetUserProfileResponse, error) {
	if req.UserId == "" {
		return &authpb.GetUserProfileResponse{
			Ok:    false,
			Error: "user_id is required",
		}, nil
	}

	user, err := h.uc.GetProfile(ctx, req.UserId)
	if err != nil {
		log.Printf("[ERROR] GetProfile failed for user_id=%s: %v", req.UserId, err)
		return &authpb.GetUserProfileResponse{
			Ok:    false,
			Error: err.Error(),
		}, nil
	}

	return &authpb.GetUserProfileResponse{
		Ok:   true,
		User: toProtoUser(user),
	}, nil
}

//
// ---------------- Get Users By Partner ----------------
//
func (h *GRPCAuthHandler) GetUsersByPartner(ctx context.Context, req *authpb.GetUsersByPartnerRequest) (*authpb.GetUsersByPartnerResponse, error) {
	if req.PartnerId == "" {
		return &authpb.GetUsersByPartnerResponse{
			Ok:    false,
			Error: "partner_id is required",
		}, nil
	}

	users, err := h.uc.GetUsersByPartnerID(ctx, req.PartnerId)
	if err != nil {
		log.Printf("[ERROR] GetUsersByPartner failed for partner_id=%s: %v", req.PartnerId, err)
		return &authpb.GetUsersByPartnerResponse{
			Ok:    false,
			Error: err.Error(),
		}, nil
	}

	respUsers := make([]*authpb.User, 0, len(users))
	for _, u := range users {
		respUsers = append(respUsers, toProtoUser(&u)) // pass pointer
	}

	return &authpb.GetUsersByPartnerResponse{
		Ok:    true,
		Users: respUsers,
	}, nil
}

//
// ---------------- Helpers ----------------
//
func toProtoUser(u *domain.User) *authpb.User {
	if u == nil {
		return nil
	}
	return &authpb.User{
		Id:              u.ID,
		Email:           getString(u.Email),
		Phone:           getString(u.Phone),
		PasswordHash:    getString(u.PasswordHash),
		FirstName:       getString(u.FirstName),
		LastName:        getString(u.LastName),
		IsEmailVerified: u.IsEmailVerified,
		IsPhoneVerified: u.IsPhoneVerified,
		IsTempPass:      u.IsTempPass,
		AccountStatus:   u.AccountStatus,
		AccountType:     u.AccountType,
		Role:            u.Role,
		CreatedAt:       u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       u.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		PartnerId:       u.PartnerID,
	}
}


func getString(ptr *string) string {
	if ptr != nil {
		return *ptr
	}
	return ""
}
