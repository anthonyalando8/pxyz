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


// handler/grpc_auth_handler.go - ADD THESE NEW METHODS

func (h *GRPCAuthHandler) GetPartnerUserStats(ctx context.Context, req *authpb.GetPartnerUserStatsRequest) (*authpb.GetPartnerUserStatsResponse, error) {
	if req.PartnerId == "" {
		return &authpb.GetPartnerUserStatsResponse{
			Ok:    false,
			Error: "partner_id is required",
		}, nil
	}

	stats, err := h.uc.GetPartnerUserStats(ctx, req.PartnerId)
	if err != nil {
		log.Printf("[ERROR] GetPartnerUserStats failed for partner_id=%s: %v", req.PartnerId, err)
		return &authpb.GetPartnerUserStatsResponse{
			Ok:    false,
			Error: err.Error(),
		}, nil
	}

	return &authpb.GetPartnerUserStatsResponse{
		Ok: true,
		Stats: &authpb.PartnerUserStats{
			PartnerId:      stats.PartnerID,
			TotalUsers:     stats.TotalUsers,
			ActiveUsers:    stats.ActiveUsers,
			SuspendedUsers: stats.SuspendedUsers,
			VerifiedUsers:  stats.VerifiedUsers,
			AdminUsers:     stats.AdminUsers,
			RegularUsers:   stats.RegularUsers,
		},
	}, nil
}

func (h *GRPCAuthHandler) GetUsersByPartnerPaginated(ctx context.Context, req *authpb.GetUsersByPartnerPaginatedRequest) (*authpb.GetUsersByPartnerPaginatedResponse, error) {
	if req.PartnerId == "" {
		return &authpb.GetUsersByPartnerPaginatedResponse{
			Ok:    false,
			Error: "partner_id is required",
		}, nil
	}

	users, total, err := h.uc.GetUsersByPartnerIDPaginated(ctx, req.PartnerId, int(req.Limit), int(req.Offset))
	if err != nil {
		log.Printf("[ERROR] GetUsersByPartnerPaginated failed: %v", err)
		return &authpb.GetUsersByPartnerPaginatedResponse{
			Ok:    false,
			Error: err.Error(),
		}, nil
	}

	respUsers := make([]*authpb.User, 0, len(users))
	for _, u := range users {
		respUsers = append(respUsers, toProtoUser(&u))
	}

	return &authpb.GetUsersByPartnerPaginatedResponse{
		Ok:         true,
		Users:      respUsers,
		TotalCount: total,
	}, nil
}

func (h *GRPCAuthHandler) UpdateUserStatus(ctx context.Context, req *authpb.UpdateUserStatusRequest) (*authpb.UpdateUserStatusResponse, error) {
	if req.UserId == "" || req.PartnerId == "" || req.Status == "" {
		return &authpb.UpdateUserStatusResponse{
			Ok:    false,
			Error: "user_id, partner_id and status are required",
		}, nil
	}

	err := h.uc.UpdateUserStatus(ctx, req.UserId, req.PartnerId, req.Status)
	if err != nil {
		log.Printf("[ERROR] UpdateUserStatus failed: %v", err)
		return &authpb.UpdateUserStatusResponse{
			Ok:    false,
			Error: err.Error(),
		}, nil
	}

	return &authpb.UpdateUserStatusResponse{Ok: true}, nil
}

func (h *GRPCAuthHandler) UpdateUserRole(ctx context.Context, req *authpb.UpdateUserRoleRequest) (*authpb.UpdateUserRoleResponse, error) {
	if req.UserId == "" || req.PartnerId == "" || req.Role == "" {
		return &authpb.UpdateUserRoleResponse{
			Ok:    false,
			Error: "user_id, partner_id and role are required",
		}, nil
	}

	err := h.uc.UpdateUserRole(ctx, req.UserId, req.PartnerId, req.Role)
	if err != nil {
		log.Printf("[ERROR] UpdateUserRole failed: %v", err)
		return &authpb.UpdateUserRoleResponse{
			Ok:    false,
			Error: err.Error(),
		}, nil
	}

	return &authpb.UpdateUserRoleResponse{Ok: true}, nil
}

func (h *GRPCAuthHandler) SearchPartnerUsers(ctx context.Context, req *authpb.SearchPartnerUsersRequest) (*authpb.SearchPartnerUsersResponse, error) {
	if req.PartnerId == "" || req.SearchTerm == "" {
		return &authpb.SearchPartnerUsersResponse{
			Ok:    false,
			Error: "partner_id and search_term are required",
		}, nil
	}

	users, err := h.uc.SearchPartnerUsers(ctx, req.PartnerId, req.SearchTerm, int(req.Limit), int(req.Offset))
	if err != nil {
		log.Printf("[ERROR] SearchPartnerUsers failed: %v", err)
		return &authpb.SearchPartnerUsersResponse{
			Ok:    false,
			Error: err.Error(),
		}, nil
	}

	respUsers := make([]*authpb.User, 0, len(users))
	for _, u := range users {
		respUsers = append(respUsers, toProtoUser(&u))
	}

	return &authpb.SearchPartnerUsersResponse{
		Ok:    true,
		Users: respUsers,
	}, nil
}

func (h *GRPCAuthHandler) GetPartnerUserByEmail(ctx context.Context, req *authpb.GetPartnerUserByEmailRequest) (*authpb.GetPartnerUserByEmailResponse, error) {
	if req.PartnerId == "" || req.Email == "" {
		return &authpb.GetPartnerUserByEmailResponse{
			Ok:    false,
			Error: "partner_id and email are required",
		}, nil
	}

	user, err := h.uc.GetPartnerUserByEmail(ctx, req.PartnerId, req.Email)
	if err != nil {
		log.Printf("[ERROR] GetPartnerUserByEmail failed: %v", err)
		return &authpb.GetPartnerUserByEmailResponse{
			Ok:    false,
			Error: err.Error(),
		}, nil
	}

	return &authpb.GetPartnerUserByEmailResponse{
		Ok:   true,
		User: toProtoUser(user),
	}, nil
}

func (h *GRPCAuthHandler) BulkUpdateUserStatus(ctx context.Context, req *authpb.BulkUpdateUserStatusRequest) (*authpb.BulkUpdateUserStatusResponse, error) {
	if req.PartnerId == "" || len(req.UserIds) == 0 || req.Status == "" {
		return &authpb.BulkUpdateUserStatusResponse{
			Ok:    false,
			Error: "partner_id, user_ids and status are required",
		}, nil
	}

	err := h.uc.BulkUpdateUserStatus(ctx, req.PartnerId, req.UserIds, req.Status)
	if err != nil {
		log.Printf("[ERROR] BulkUpdateUserStatus failed: %v", err)
		return &authpb.BulkUpdateUserStatusResponse{
			Ok:    false,
			Error: err.Error(),
		}, nil
	}

	return &authpb.BulkUpdateUserStatusResponse{Ok: true}, nil
}