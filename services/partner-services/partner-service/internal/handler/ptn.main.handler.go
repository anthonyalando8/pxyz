package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	"x/shared/utils/id"

	domain "partner-service/internal/domain"
	"partner-service/internal/usecase"

	authclient "x/shared/auth" // gRPC/HTTP client for auth-service
	otpclient "x/shared/auth/otp"
	"x/shared/response"
	"x/shared/auth/middleware"

	accountingclient "x/shared/common/accounting"
	accountingpb "x/shared/genproto/shared/accounting/accountingpb"
	authpb "x/shared/genproto/partner/authpb"

	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PartnerHandler struct {
	uc              *usecase.PartnerUsecase
	authClient      *authclient.AuthService
	otp             *otpclient.OTPService
	emailClient     *emailclient.EmailClient
	smsClient       *smsclient.SMSClient
	accountingClient *accountingclient.AccountingClient
}

func NewPartnerHandler(
	uc *usecase.PartnerUsecase,
	authClient *authclient.AuthService,
	otp *otpclient.OTPService,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
	accountingClient *accountingclient.AccountingClient,
) *PartnerHandler {
	return &PartnerHandler{
		uc:              uc,
		authClient:      authClient,
		otp:             otp,
		emailClient:     emailClient,
		smsClient:       smsClient,
		accountingClient: accountingClient,
	}
}


func decodeJSON(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}


// ----------------- ACCOUNTING ENDPOINTS -----------------

// GetUserAccounts returns all accounts for the partner linked to the current user
func (h *PartnerHandler) GetUserAccounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// extract user id
	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "missing or invalid user ID")
		return
	}

	// fetch partner ID from profile
	profileResp, err := h.authClient.PartnerClient.GetUserProfile(ctx, &authpb.GetUserProfileRequest{
		UserId: userID,
	})
	if err != nil || profileResp == nil || profileResp.User == nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch user profile from auth service")
		return
	}
	partnerID := profileResp.User.PartnerId
	if partnerID == "" {
		response.Error(w, http.StatusForbidden, "your account is not linked to a partner")
		return
	}

	req := &accountingpb.GetAccountsRequest{
		OwnerType: accountingpb.OwnerType_PARTNER,
		OwnerId:   partnerID,
	}

	resp, err := h.accountingClient.Client.GetUserAccounts(ctx, req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to fetch accounts: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GetAccountStatement fetches statement for a specific account
func (h *PartnerHandler) GetAccountStatement(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AccountNumber string     `json:"account_number"`
		From      time.Time `json:"from"`
		To        time.Time `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req := &accountingpb.AccountStatementRequest{
		AccountNumber: in.AccountNumber,
		From:      timestamppb.New(in.From),
		To:        timestamppb.New(in.To),
	}

	resp, err := h.accountingClient.Client.GetAccountStatement(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to fetch account statement: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GetOwnerStatement fetches all account statements for the current partner
func (h *PartnerHandler) GetOwnerStatement(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// extract user id
	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "missing or invalid user ID")
		return
	}

	// fetch partner ID
	profileResp, err := h.authClient.PartnerClient.GetUserProfile(ctx, &authpb.GetUserProfileRequest{
		UserId: userID,
	})
	if err != nil || profileResp == nil || profileResp.User == nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch user profile from auth service")
		return
	}
	partnerID := profileResp.User.PartnerId
	if partnerID == "" {
		response.Error(w, http.StatusForbidden, "your account is not linked to a partner")
		return
	}

	var in struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req := &accountingpb.OwnerStatementRequest{
		OwnerType: accountingpb.OwnerType_PARTNER,
		OwnerId:   partnerID,
		From:      timestamppb.New(in.From),
		To:        timestamppb.New(in.To),
	}

	stream, err := h.accountingClient.Client.GetOwnerStatement(ctx, req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to request owner statement: "+err.Error())
		return
	}

	var results []*accountingpb.AccountStatement
	for {
		stmt, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			response.Error(w, http.StatusBadGateway, "error receiving owner statement stream: "+err.Error())
			return
		}
		results = append(results, stmt)
	}

	response.JSON(w, http.StatusOK, results)
}




// CreatePartnerUser (calls auth service to create user first)
func (h *PartnerHandler) CreatePartnerUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}

	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// --- Step 1: Get current authenticated user profile from Auth service ---
	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "missing or invalid user ID")
		return
	}

	// Ask Auth service for profile (to fetch PartnerID)
	profileResp, err := h.authClient.PartnerClient.GetUserProfile(ctx, &authpb.GetUserProfileRequest{
		UserId: userID,
	})
	if err != nil || profileResp == nil || profileResp.User == nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch user profile from auth service")
		return
	}
	partnerID := profileResp.User.PartnerId
	if partnerID == "" {
		response.Error(w, http.StatusForbidden, "your account is not linked to a partner")
		return
	}

	// --- Step 2: Generate password if missing ---
	if req.Password == "" {
		var err error
		req.Password, err = id.GeneratePassword()
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to generate password: "+err.Error())
			return
		}
	}

	// --- Step 3: Create user in Auth service ---
	userResp, err := h.authClient.PartnerClient.RegisterUser(ctx, &authpb.RegisterUserRequest{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      string(domain.PartnerUserRoleUser), // defaults to "user"
		PartnerId: partnerID,                          // derived, not provided by client
	})

	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to call auth service: "+err.Error())
		return
	}
	if userResp == nil || !userResp.Ok {
		errorMsg := "unknown error"
		if userResp != nil {
			errorMsg = userResp.Error
		}
		response.Error(w, http.StatusConflict, "failed to create user: "+errorMsg)
		return
	}

	// --- Step 4: Build domain PartnerUser (mirror) ---
	partnerUser := &domain.PartnerUser{
		ID:        userResp.UserId,
		PartnerID: partnerID,
		Email:     req.Email,
		Role:      domain.PartnerUserRoleUser,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// --- Step 5: Send notifications ---
	sendNewPartnerUserNotifications(ctx, h.uc, h.emailClient, partnerID, userResp.UserId, partnerUser, req.Password)

	// --- Step 6: Respond ---
	response.JSON(w, http.StatusCreated, partnerUser)
}


func (h *PartnerHandler) UpdatePartnerUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	paramID := chi.URLParam(r, "id")
	ctxID, _ := ctx.Value(middleware.ContextUserID).(string)
	ctxRole, _ := ctx.Value(middleware.ContextRole).(string)

	// --- Step 1: Resolve target user ID ---
	var targetID string
	if paramID == "" {
		// no param → user is updating their own account
		if ctxID == "" {
			response.Error(w, http.StatusUnauthorized, "missing user ID in context")
			return
		}
		targetID = ctxID
	} else if paramID != ctxID {
		// param exists and does not match context ID → check if caller is admin
		if ctxRole != string(domain.PartnerUserRoleAdmin) {
			response.Error(w, http.StatusForbidden, "not allowed to update another user account")
			return
		}
		targetID = paramID
	} else {
		// param exists and equals context ID
		targetID = ctxID
	}

	// --- Step 2: Parse request body ---
	var req struct {
		Role     string `json:"role"`      // expecting "partner_admin" | "partner_user"
		IsActive bool   `json:"is_active"`
	}
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// --- Step 2a: Protect role changes (server-side) ---
	if req.Role != "" {
		// Only partner_admin may change roles
		if ctxRole != string(domain.PartnerUserRoleAdmin) {
			response.Error(w, http.StatusForbidden, "only partner_admin can change roles")
			return
		}
		// Validate the role value
		if req.Role != string(domain.PartnerUserRoleAdmin) && req.Role != string(domain.PartnerUserRoleUser) {
			response.Error(w, http.StatusBadRequest, "invalid role value")
			return
		}
	}

	// --- Step 3: Build domain object (partial update semantics expected in usecase) ---
	partnerUser := &domain.PartnerUser{
		ID:       targetID,
		Role:     domain.PartnerUserRole(req.Role), // empty string means "no change" (usecase should handle)
		IsActive: req.IsActive,
	}

	// --- Step 4: Perform update ---
	if err := h.uc.UpdatePartnerUser(ctx, partnerUser); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// --- Step 5: Notifications & Response ---
	sendPartnerUpdatedNotification(ctx, partnerUser.ID)
	response.JSON(w, http.StatusOK, partnerUser)
}

func (h *PartnerHandler) DeletePartnerUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// --- 1. Extract target user ID ---
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "missing partner_user id")
		return
	}

	// --- 2. Authorisation: caller must be partner_admin ---
	ctxRole, _ := ctx.Value(middleware.ContextRole).(string)
	if ctxRole != string(domain.PartnerUserRoleAdmin) {
		response.Error(w, http.StatusForbidden, "only partner_admin can delete users")
		return
	}

	// --- 3. Fetch latest user profile from PartnerAuthService ---
	profileResp, err := h.authClient.PartnerClient.GetUserProfile(ctx, &authpb.GetUserProfileRequest{
		UserId: id,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch user profile: "+err.Error())
		return
	}
	if profileResp == nil || !profileResp.Ok || profileResp.User == nil {
		response.Error(w, http.StatusNotFound, "partner_user not found in auth service")
		return
	}

	user := profileResp.User

	// --- 4. Ensure target is partner_user, not partner_admin ---
	if user.Role != string(domain.PartnerUserRoleUser) {
		response.Error(w, http.StatusForbidden, "cannot delete admin users")
		return
	}

	// --- 5. Delete user via PartnerAuthService ---
	delResp, err := h.authClient.PartnerClient.DeleteUser(ctx, &authpb.DeleteUserRequest{
		UserId: id,
	})
	if err != nil || delResp == nil || !delResp.Ok {
		msg := "unknown error"
		if err != nil {
			msg = err.Error()
		} else if delResp != nil {
			msg = delResp.Error
		}
		response.Error(w, http.StatusInternalServerError, "failed to delete user: "+msg)
		return
	}

	// --- 6. Notify & Respond ---
	sendPartnerDeletedNotification(ctx, id)

	response.JSON(w, http.StatusOK, map[string]string{
		"deleted_id": id,
	})
}
