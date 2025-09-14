package handler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	domain "partner-service/internal/domain"
	"partner-service/internal/usecase"
	partnerauthpb "x/shared/genproto/partner/authpb"
	partnersvcpb "x/shared/genproto/partner/svcpb"

	authclient "x/shared/auth"
	otpclient "x/shared/auth/otp"
	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	"x/shared/utils/id"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCPartnerHandler implements PartnerService gRPC methods
type GRPCPartnerHandler struct {
	partnersvcpb.UnimplementedPartnerServiceServer

	uc          *usecase.PartnerUsecase
	authClient  *authclient.AuthService
	otp         *otpclient.OTPService
	emailClient *emailclient.EmailClient
	smsClient   *smsclient.SMSClient
}

// constructor
func NewGRPCPartnerHandler(
	uc *usecase.PartnerUsecase,
	authClient *authclient.AuthService,
	otp *otpclient.OTPService,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
) *GRPCPartnerHandler {
	return &GRPCPartnerHandler{
		uc:          uc,
		authClient:  authClient,
		otp:         otp,
		emailClient: emailClient,
		smsClient:   smsClient,
	}
}
func (h *GRPCPartnerHandler) CreatePartner(
	ctx context.Context,
	req *partnersvcpb.CreatePartnerRequest,
) (*partnersvcpb.PartnerResponse, error) {
	// --- 1. Build partner domain object ---
	partner := &domain.Partner{
		ID:           id.GenerateID("PTN"),
		Name:         req.Name,
		Country:      req.Country,
		ContactEmail: req.ContactEmail,
		ContactPhone: req.ContactPhone,
		Status:       domain.PartnerStatusActive,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// --- 2. Persist partner in DB ---
	if err := h.uc.CreatePartner(ctx, partner); err != nil {
		log.Printf("[ERROR] CreatePartner failed: %v", err)
		return nil, err
	}

	// --- 3. Generate password for default admin ---
	password, err := id.GeneratePassword()
	if err != nil {
		log.Printf("[ERROR] Failed to generate password: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to generate admin password")
	}

	// --- 4. Create default admin user in Auth service ---
	userResp, err := h.authClient.PartnerClient.RegisterUser(ctx, &partnerauthpb.RegisterUserRequest{
		Email:     partner.ContactEmail, // default admin uses partner contact email
		Password:  password,
		FirstName: partner.Name,
		LastName:  "Admin",
		Role:      "partner_admin",
		PartnerId: partner.ID,
	})
	if err != nil || userResp == nil {
		log.Printf("[ERROR] Failed to create default partner admin for partner=%s: %v", partner.ID, err)
		return nil, status.Errorf(codes.Internal, "failed to create default admin account")
	}
	if !userResp.Ok {
		log.Printf("[ERROR] Auth service rejected default admin creation: %s", userResp.Error)
		return nil, status.Errorf(codes.AlreadyExists, "failed to create default admin: %s", userResp.Error)
	}

	sendPartnerCreatedEmail(
		h.uc,
		h.emailClient,
		partner,
		password,
	)

	return &partnersvcpb.PartnerResponse{
		Partner: partner.ToProto(),
	}, nil
}




func (h *GRPCPartnerHandler) UpdatePartner(ctx context.Context, req *partnersvcpb.UpdatePartnerRequest) (*partnersvcpb.PartnerResponse, error) {
	partner := &domain.Partner{
		ID:           req.Id,
		Name:         req.Name,
		Country:      req.Country,
		ContactEmail: req.ContactEmail,
		ContactPhone: req.ContactPhone,
		Status:       domain.PartnerStatus(req.Status), // convert string to PartnerStatus
	}

	if err := h.uc.UpdatePartner(ctx, partner); err != nil {
		log.Printf("[ERROR] UpdatePartner failed: %v", err)
		return nil, err
	}

	return &partnersvcpb.PartnerResponse{
		Partner: partner.ToProto(),
	}, nil
}


func (h *GRPCPartnerHandler) DeletePartner(ctx context.Context, req *partnersvcpb.DeletePartnerRequest) (*partnersvcpb.DeletePartnerResponse, error) {
	if err := h.uc.DeletePartner(ctx, req.Id); err != nil {
		log.Printf("[ERROR] DeletePartner failed: %v", err)
		return &partnersvcpb.DeletePartnerResponse{Success: false}, err
	}
	return &partnersvcpb.DeletePartnerResponse{Success: true}, nil
}

func (h *GRPCPartnerHandler) CreatePartnerUser(
	ctx context.Context,
	req *partnersvcpb.CreatePartnerUserRequest,
) (*partnersvcpb.PartnerUserResponse, error) {

	// 1. Generate password if missing
	password := ""
	if password == "" {
		var err error
		password, err = id.GeneratePassword()
		if err != nil {
			log.Printf("[ERROR] failed to generate password: %v", err)
			return nil, fmt.Errorf("failed to generate password: %w", err)
		}
	}

	// 2. Call PartnerAuthService to register
	userResp, err := h.authClient.PartnerClient.RegisterUser(ctx, &partnerauthpb.RegisterUserRequest{
		Email:     req.Email,
		Password:  password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      req.Role,
		PartnerId: req.PartnerId,
	})
	if err != nil {
		return nil, fmt.Errorf("auth service RPC failed: %w", err)
	}
	if userResp == nil || !userResp.Ok {
		return nil, fmt.Errorf("auth service error: %s", userResp.GetError())
	}

	// 3. Build domain model
	domainUser := &domain.PartnerUser{
		ID:        userResp.UserId,
		PartnerID: req.PartnerId,
		Role:      domain.PartnerUserRole(req.Role),
		Email:     req.Email,
		UserID:    userResp.UserId, // mirrors ID, unless you keep separate IDs
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 4. Send notifications (use domain struct here)
	sendNewPartnerUserNotifications(ctx, h.uc, h.emailClient, req.PartnerId, userResp.UserId, domainUser, password)

	// 5. Map to proto for response
	protoUser := &partnersvcpb.PartnerUser{
		Id:            domainUser.ID,
		PartnerId:     domainUser.PartnerID,
		Email:         domainUser.Email,
		Role:          string(domainUser.Role),
		AccountStatus: "active",
		CreatedAt:     timestamppb.New(domainUser.CreatedAt),
		UpdatedAt:     timestamppb.New(domainUser.UpdatedAt),
	}

	return &partnersvcpb.PartnerUserResponse{User: protoUser}, nil
}



func (h *GRPCPartnerHandler) DeletePartnerUsers(
    ctx context.Context,
    req *partnersvcpb.DeletePartnerUsersRequest,
) (*partnersvcpb.DeletePartnerUsersResponse, error) {

    var (
        deletedIDs  []string
        failedUsers []*partnersvcpb.FailedUserDeletion
        mu          sync.Mutex
        wg          sync.WaitGroup
    )

    sem := make(chan struct{}, 10) // limit to 10 concurrent deletions

    for _, userID := range req.UserIds {
        wg.Add(1)
        go func(uid string) {
            defer wg.Done()
            sem <- struct{}{} // acquire semaphore
            defer func() { <-sem }() // release semaphore

            delResp, err := h.authClient.PartnerClient.DeleteUser(ctx, &partnerauthpb.DeleteUserRequest{UserId: uid})

            mu.Lock()
            defer mu.Unlock()

            if err != nil {
                log.Printf("[WARN] DeletePartnerUsers failed for user_id=%s: %v", uid, err)
                failedUsers = append(failedUsers, &partnersvcpb.FailedUserDeletion{
                    UserId: uid,
                    Reason: fmt.Sprintf("gRPC error: %v", err),
                })
                return
            }

            if !delResp.Ok {
                log.Printf("[WARN] DeletePartnerUsers failed for user_id=%s: %s", uid, delResp.Error)
                failedUsers = append(failedUsers, &partnersvcpb.FailedUserDeletion{
                    UserId: uid,
                    Reason: delResp.Error,
                })
                return
            }

            deletedIDs = append(deletedIDs, uid)
        }(userID)
    }

    wg.Wait()

    return &partnersvcpb.DeletePartnerUsersResponse{
        DeletedIds: deletedIDs,
        FailedUsers: failedUsers, // include reason per user
    }, nil
}

