package handler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	domain "partner-service/internal/domain"
	"partner-service/internal/usecase"
	partnerauthpb "x/shared/genproto/partner/authpb"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	accountingpb "x/shared/genproto/shared/accounting/v1"
	accountingclient "x/shared/common/accounting"

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

	uc               *usecase. PartnerUsecase
	authClient       *authclient.AuthService
	otp              *otpclient.OTPService
	emailClient      *emailclient.EmailClient
	smsClient        *smsclient. SMSClient
	accountingClient *accountingclient.AccountingClient
}

// constructor
func NewGRPCPartnerHandler(
	uc *usecase.PartnerUsecase,
	authClient *authclient.AuthService,
	otp *otpclient.OTPService,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
	accountingClient *accountingclient. AccountingClient,
) *GRPCPartnerHandler {
	return &GRPCPartnerHandler{
		uc:                uc,
		authClient:       authClient,
		otp:              otp,
		emailClient:      emailClient,
		smsClient:        smsClient,
		accountingClient:  accountingClient,
	}
}

func (h *GRPCPartnerHandler) CreatePartner(
	ctx context.Context,
	req *partnersvcpb. CreatePartnerRequest,
) (*partnersvcpb.PartnerResponse, error) {
	// ✅ Validate required fields from proto
	if req.Name == "" {
		return nil, status. Errorf(codes.InvalidArgument, "name is required")
	}
	if req.Service == "" {
		return nil, status.Errorf(codes. InvalidArgument, "service is required")
	}
	if req.Currency == "" {
		return nil, status.Errorf(codes.InvalidArgument, "currency is required")
	}
	
	// ✅ Validate local_currency and rate from proto
	if req.LocalCurrency == "" {
		return nil, status.Errorf(codes.InvalidArgument, "local_currency is required")
	}
	if req.Rate <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "rate must be greater than 0")
	}

	// Convert to lowercase if the identifier looks like an email
	req.ContactEmail = strings.ToLower(req. ContactEmail)
	
	// ✅ Build partner domain object with ALL required fields
	partner := &domain. Partner{
		ID:             id.GenerateID("PTN"),
		Name:           req.Name,
		Country:        req. Country,
		ContactEmail:   req.ContactEmail,
		ContactPhone:   req.ContactPhone,
		Status:         domain. PartnerStatusActive,
		Service:        req.Service,
		Currency:       req.Currency,
		LocalCurrency:  req.LocalCurrency,     // ✅ Required
		Rate:           req.Rate,              // ✅ Required
		CommissionRate: req.CommissionRate,    // ✅ Optional but included
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Persist partner in DB (will validate and generate API credentials)
	if err := h.uc.CreatePartner(ctx, partner); err != nil {
		log.Printf("[ERROR] CreatePartner failed: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to create partner:  %v", err)
	}

	// Generate password for default admin
	password, err := id.GeneratePassword()
	if err != nil {
		log.Printf("[ERROR] Failed to generate password: %v", err)
		return nil, status. Errorf(codes.Internal, "failed to generate admin password")
	}

	// Create default admin user in Auth service (async, resilient)
	go func(p *domain.Partner, pwd string) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC RECOVERED] Auth goroutine crashed for partner=%s: %v", p.ID, r)
			}
		}()

		ctxBg, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		userResp, err := h.authClient. PartnerClient.RegisterUser(ctxBg, &partnerauthpb.RegisterUserRequest{
			Email:     p.ContactEmail,
			Password:  pwd,
			FirstName:  p.Name,
			LastName:  "Admin",
			Role:      "partner_admin",
			PartnerId: p.ID,
		})
		if err != nil {
			log.Printf("[ERROR] Failed to create default partner admin for partner=%s: %v", p.ID, err)
			return
		}
		if userResp == nil {
			log.Printf("[ERROR] Nil response from Auth service for partner=%s", p.ID)
			return
		}
		if !userResp.Ok {
			log.Printf("[ERROR] Auth service rejected default admin creation for partner=%s: %s", p.ID, userResp. Error)
			return
		}

		// Send notification email
		sendPartnerCreatedEmail(h.uc, h.emailClient, p, pwd)

		log.Printf("[INFO] Default partner admin created + email sent for partner=%s", p.ID)
	}(partner, password)

	// Create settlement account in Accounting service (async, resilient)
	go func(p *domain.Partner) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC RECOVERED] Accounting goroutine crashed for partner=%s:  %v", p.ID, r)
			}
		}()

		ctxBg, cancel := context.WithTimeout(context. Background(), 15*time.Second)
		defer cancel()

		// ✅ Create settlement account for partner's currency
		reqAcc := &accountingpb. CreateAccountsRequest{
			Accounts: []*accountingpb.CreateAccountRequest{
				{
					OwnerType:   accountingpb.OwnerType_OWNER_TYPE_PARTNER,
					OwnerId:     p.ID,
					Currency:    p.Currency,        // ✅ Partner's working currency
					Purpose:     accountingpb.AccountPurpose_ACCOUNT_PURPOSE_SETTLEMENT,
					AccountType: accountingpb.AccountType_ACCOUNT_TYPE_REAL,
				},
			},
		}

		resp, err := h.accountingClient.Client.CreateAccounts(ctxBg, reqAcc)
		if err != nil {
			log.Printf("[ERROR] Failed to create settlement account for partner=%s: %v", p.ID, err)
			return
		}
		if resp == nil {
			log.Printf("[ERROR] Nil response from Accounting service for partner=%s", p.ID)
			return
		}

		// Check for errors in creation
		if len(resp. Errors) > 0 {
			log.Printf("[ERROR] Failed to create settlement account for partner=%s: %+v", p.ID, resp. Errors)
			return
		}

		if len(resp.Accounts) == 0 {
			log.Printf("[WARN] Accounting service returned no accounts for partner=%s", p.ID)
			return
		}

		// Log successful account creation
		acc := resp.Accounts[0]
		log.Printf("[INFO] ✅ Created settlement account for partner=%s:  accountId=%d, accountNumber=%s, currency=%s, rate=%f %s/%s",
			p.ID, acc.Id, acc.AccountNumber, acc.Currency, p.Rate, p.LocalCurrency, p.Currency)
	}(partner)

	// Respond immediately
	return &partnersvcpb.PartnerResponse{
		Partner: partner.ToProto(),
	}, nil
}

func (h *GRPCPartnerHandler) UpdatePartner(
	ctx context. Context,
	req *partnersvcpb.UpdatePartnerRequest,
) (*partnersvcpb.PartnerResponse, error) {
	// ✅ Validate ID
	if req.Id == "" {
		return nil, status.Errorf(codes. InvalidArgument, "partner id is required")
	}

	// Convert to lowercase if the identifier looks like an email
	req.ContactEmail = strings.ToLower(req. ContactEmail)
	
	// ✅ Build partner with all updatable fields
	partner := &domain.Partner{
		ID:             req.Id,
		Name:           req.Name,
		Country:        req.Country,
		ContactEmail:   req.ContactEmail,
		ContactPhone:   req.ContactPhone,
		Status:         domain.PartnerStatus(req.Status),
		Service:        req.Service,
		Currency:       req. Currency,
		LocalCurrency:   req.LocalCurrency,     // ✅ Can be updated
		Rate:           req.Rate,              // ✅ Can be updated
		CommissionRate: req.CommissionRate,    // ✅ Can be updated
	}

	if err := h.uc.UpdatePartner(ctx, partner); err != nil {
		log.Printf("[ERROR] UpdatePartner failed: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to update partner: %v", err)
	}

	// ✅ Fetch updated partner to return complete data
	updatedPartner, err := h. uc.GetPartnerByID(ctx, req.Id)
	if err != nil {
		log.Printf("[ERROR] Failed to fetch updated partner: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to fetch updated partner")
	}

	return &partnersvcpb.PartnerResponse{
		Partner: updatedPartner.ToProto(),
	}, nil
}

func (h *GRPCPartnerHandler) DeletePartner(
	ctx context.Context,
	req *partnersvcpb.DeletePartnerRequest,
) (*partnersvcpb.DeletePartnerResponse, error) {
	if req.Id == "" {
		return nil, status.Errorf(codes.InvalidArgument, "partner id is required")
	}

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
	// Validate required fields
	if req. PartnerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "partner_id is required")
	}
	if req.Email == "" && req.Phone == "" {
		return nil, status.Errorf(codes.InvalidArgument, "email or phone is required")
	}

	// Generate password
	password, err := id.GeneratePassword()
	if err != nil {
		log.Printf("[ERROR] failed to generate password: %v", err)
		return nil, fmt.Errorf("failed to generate password: %w", err)
	}

	// Convert to lowercase if the identifier looks like an email
	req.Email = strings.ToLower(req.Email)
	
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

	// Build domain model
	domainUser := &domain.PartnerUser{
		ID:        userResp.UserId,
		PartnerID: req.PartnerId,
		Role:      domain.PartnerUserRole(req.Role),
		Email:     req.Email,
		UserID:    userResp.UserId,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Send notifications
	sendNewPartnerUserNotifications(ctx, h.uc, h.emailClient, req. PartnerId, userResp. UserId, domainUser, password)

	// Map to proto for response
	protoUser := &partnersvcpb.PartnerUser{
		Id:            domainUser.ID,
		PartnerId:     domainUser. PartnerID,
		Email:         domainUser.Email,
		Role:          string(domainUser.Role),
		AccountStatus: "active",
		CreatedAt:      timestamppb.New(domainUser.CreatedAt),
		UpdatedAt:     timestamppb.New(domainUser. UpdatedAt),
	}

	return &partnersvcpb.PartnerUserResponse{User: protoUser}, nil
}

func (h *GRPCPartnerHandler) DeletePartnerUsers(
	ctx context.Context,
	req *partnersvcpb.DeletePartnerUsersRequest,
) (*partnersvcpb.DeletePartnerUsersResponse, error) {
	var (
		deletedIDs  []string
		failedUsers []*partnersvcpb. FailedUserDeletion
		mu          sync. Mutex
		wg          sync.WaitGroup
	)

	sem := make(chan struct{}, 10) // limit to 10 concurrent deletions

	for _, userID := range req.UserIds {
		wg.Add(1)
		go func(uid string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

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

			if ! delResp.Ok {
				log.Printf("[WARN] DeletePartnerUsers failed for user_id=%s: %s", uid, delResp.Error)
				failedUsers = append(failedUsers, &partnersvcpb.FailedUserDeletion{
					UserId: uid,
					Reason:  delResp.Error,
				})
				return
			}

			deletedIDs = append(deletedIDs, uid)
		}(userID)
	}

	wg.Wait()

	return &partnersvcpb.DeletePartnerUsersResponse{
		DeletedIds:   deletedIDs,
		FailedUsers: failedUsers,
	}, nil
}

func (h *GRPCPartnerHandler) GetPartners(
	ctx context.Context,
	req *partnersvcpb.GetPartnersRequest,
) (*partnersvcpb.GetPartnersResponse, error) {
	partnerIDs := req.GetPartnerIds()

	partners, err := h.uc.GetPartners(ctx, partnerIDs)
	if err != nil {
		log.Printf("[ERROR] GetPartners failed: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to fetch partners")
	}

	protoPartners := make([]*partnersvcpb.Partner, 0, len(partners))
	for _, p := range partners {
		protoPartners = append(protoPartners, p.ToProto())
	}

	return &partnersvcpb.GetPartnersResponse{
		Partners: protoPartners,
	}, nil
}

func (h *GRPCPartnerHandler) GetPartnersByService(
	ctx context.Context,
	req *partnersvcpb.GetPartnersByServiceRequest,
) (*partnersvcpb.GetPartnersResponse, error) {
	service := req.GetService()
	if service == "" {
		return nil, status.Errorf(codes.InvalidArgument, "service is required")
	}

	partners, err := h.uc. GetPartnersByService(ctx, service)
	if err != nil {
		log.Printf("[ERROR] GetPartnersByService failed for service=%s: %v", service, err)
		return nil, status.Errorf(codes. Internal, "failed to fetch partners by service")
	}

	protoPartners := make([]*partnersvcpb.Partner, 0, len(partners))
	for _, p := range partners {
		protoPartners = append(protoPartners, p.ToProto())
	}

	return &partnersvcpb. GetPartnersResponse{
		Partners: protoPartners,
	}, nil
}

func (h *GRPCPartnerHandler) StreamAllPartners(
	req *partnersvcpb.StreamAllPartnersRequest,
	stream partnersvcpb. PartnerService_StreamAllPartnersServer,
) error {
	batchSize := int(req.BatchSize)
	if batchSize <= 0 {
		batchSize = 1000
	}

	ctx := stream.Context()

	return h.uc.StreamAllPartners(ctx, batchSize, func(p *domain.Partner) error {
		return stream.Send(&partnersvcpb.Partner{
			Id:             p. ID,
			Name:           p.Name,
			Country:        p.Country,
			ContactEmail:   p.ContactEmail,
			ContactPhone:   p.ContactPhone,
			Status:         string(p.Status),
			Service:        p.Service,
			Currency:       p.Currency,
			LocalCurrency:  p. LocalCurrency,     // ✅ Added
			Rate:           p.Rate,              // ✅ Added (float64 string representation)
			InverseRate:    p.InverseRate,       // ✅ Added
			CommissionRate:  p.CommissionRate,    // ✅ Added
			ApiKey:         safeString(p.APIKey),
			IsApiEnabled:   p.IsAPIEnabled,
			ApiRateLimit:   p.APIRateLimit,
			WebhookUrl:     safeString(p.WebhookURL),
			CallbackUrl:    safeString(p.CallbackURL),
			CreatedAt:      timestamppb.New(p. CreatedAt),
			UpdatedAt:      timestamppb. New(p.UpdatedAt),
		})
	})
}

func toPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func safeString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}