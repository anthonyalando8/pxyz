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
	accountingclient "x/shared/common/accounting" // 👈 added
	accountingpb "x/shared/genproto/shared/accounting/accountingpb"

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
	accountingClient *accountingclient.AccountingClient // 👈 added
}

// constructor
func NewGRPCPartnerHandler(
	uc *usecase.PartnerUsecase,
	authClient *authclient.AuthService,
	otp *otpclient.OTPService,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
	accountingClient *accountingclient.AccountingClient,
) *GRPCPartnerHandler {
	return &GRPCPartnerHandler{
		uc:          uc,
		authClient:  authClient,
		otp:         otp,
		emailClient: emailClient,
		smsClient:   smsClient,
		accountingClient: accountingClient,
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
		Service:      req.Service,   // new field from proto
		Currency:     req.Currency,  // new field from proto
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
	go func(p *domain.Partner, pwd string) {
		ctxBg, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		userResp, err := h.authClient.PartnerClient.RegisterUser(ctxBg, &partnerauthpb.RegisterUserRequest{
			Email:     p.ContactEmail, // default admin uses partner contact email
			Password:  pwd,
			FirstName: p.Name,
			LastName:  "Admin",
			Role:      "partner_admin",
			PartnerId: p.ID,
		})
		if err != nil || userResp == nil {
			log.Printf("[ERROR] Failed to create default partner admin for partner=%s: %v", p.ID, err)
			return
		}
		if !userResp.Ok {
			log.Printf("[ERROR] Auth service rejected default admin creation for partner=%s: %s", p.ID, userResp.Error)
			return
		}

		// --- Send notification email ---
		sendPartnerCreatedEmail(h.uc, h.emailClient, p, pwd)

		log.Printf("[INFO] Default partner admin created + email sent for partner=%s", p.ID)
	}(partner, password)

	// --- 6. Create default account in Accounting service (async) ---
	go func() {
		ctxBg, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		reqAcc := &accountingpb.CreateAccountRequest{
			Accounts: []*accountingpb.Account{
				{
					OwnerType:   accountingpb.OwnerType_PARTNER,
					OwnerId:     partner.ID,
					Currency:    partner.Currency, // use partner's currency
					Purpose:     "wallet",
					AccountType: "real", // or "demo" if applicable
					IsActive:    true,
				},
			},
		}

		resp, err := h.accountingClient.Client.CreateAccounts(ctxBg, reqAcc)
		if err != nil {
			log.Printf("[ERROR] Failed to create default account for partner=%s: %v", partner.ID, err)
			//return
		}

		if len(resp.Errors) > 0 {
			log.Printf("[WARN] Errors creating account(s) for partner=%s: %+v", partner.ID, resp.Errors)
		} else {
			log.Printf("[INFO] Created default account for partner=%s accountId=%d", partner.ID, resp.Accounts[0].Id)
		}
	}()

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


// GetPartners handles fetching partners (optionally filtered by IDs)
func (h *GRPCPartnerHandler) GetPartners(
	ctx context.Context,
	req *partnersvcpb.GetPartnersRequest,
) (*partnersvcpb.GetPartnersResponse, error) {
	partnerIDs := req.GetPartnerIds() // slice of IDs; may be empty

	partners, err := h.uc.GetPartners(ctx, partnerIDs)
	if err != nil {
		log.Printf("[ERROR] GetPartners failed: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to fetch partners")
	}

	// Convert domain partners to proto
	protoPartners := make([]*partnersvcpb.Partner, 0, len(partners))
	for _, p := range partners {
		protoPartners = append(protoPartners, p.ToProto())
	}

	return &partnersvcpb.GetPartnersResponse{
		Partners: protoPartners,
	}, nil
}

// // GetPartnerUsers handles fetching all users under a specific partner
// func (h *GRPCPartnerHandler) GetPartnerUsers(
// 	ctx context.Context,
// 	req *partnersvcpb.GetPartnerUsersRequest,
// ) (*partnersvcpb.GetPartnerUsersResponse, error) {
// 	partnerID := req.GetPartnerId()
// 	if partnerID == "" {
// 		return nil, status.Errorf(codes.InvalidArgument, "partner_id is required")
// 	}

// 	users, err := h.uc.GetPartnerUsers(ctx, partnerID)
// 	if err != nil {
// 		log.Printf("[ERROR] GetPartnerUsers failed for partner=%s: %v", partnerID, err)
// 		return nil, status.Errorf(codes.Internal, "failed to fetch partner users")
// 	}

// 	// Convert domain users to proto
// 	protoUsers := make([]*partnersvcpb.PartnerUser, 0, len(users))
// 	for _, u := range users {
// 		protoUsers = append(protoUsers, u.ToProto())
// 	}

// 	return &partnersvcpb.GetPartnerUsersResponse{
// 		Users: protoUsers,
// 	}, nil
// }


func (h *GRPCPartnerHandler) GetPartnersByService(
	ctx context.Context,
	req *partnersvcpb.GetPartnersByServiceRequest,
) (*partnersvcpb.GetPartnersResponse, error) {
	service := req.GetService()
	if service == "" {
		return nil, status.Errorf(codes.InvalidArgument, "service is required")
	}

	partners, err := h.uc.GetPartnersByService(ctx, service)
	if err != nil {
		log.Printf("[ERROR] GetPartnersByService failed for service=%s: %v", service, err)
		return nil, status.Errorf(codes.Internal, "failed to fetch partners by service")
	}

	protoPartners := make([]*partnersvcpb.Partner, 0, len(partners))
	for _, p := range partners {
		protoPartners = append(protoPartners, p.ToProto())
	}

	return &partnersvcpb.GetPartnersResponse{
		Partners: protoPartners,
	}, nil
}
