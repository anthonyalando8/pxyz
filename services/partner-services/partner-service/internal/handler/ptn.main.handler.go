package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	"x/shared/utils/id"

	domain "partner-service/internal/domain"
	"partner-service/internal/usecase"

	authclient "x/shared/auth" // gRPC/HTTP client for auth-service
	"x/shared/auth/middleware"
	otpclient "x/shared/auth/otp"
	"x/shared/response"

	accountingclient "x/shared/common/accounting"
	authpb "x/shared/genproto/partner/authpb"
	accountingpb "x/shared/genproto/shared/accounting/accountingpb"

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
	// Convert to lowercase if the identifier looks like an email
	req.Email = strings.ToLower(req.Email)

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


// handler/partner_handler.go - ADD THESE NEW METHODS

// GetPartnerUserStats returns statistics about users under the current partner
func (h *PartnerHandler) GetPartnerUserStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "missing or invalid user ID")
		return
	}

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

	statsResp, err := h.authClient.PartnerClient.GetPartnerUserStats(ctx, &authpb.GetPartnerUserStatsRequest{
		PartnerId: partnerID,
	})
	if err != nil || statsResp == nil || !statsResp.Ok {
		msg := "unknown error"
		if err != nil {
			msg = err.Error()
		} else if statsResp != nil {
			msg = statsResp.Error
		}
		response.Error(w, http.StatusInternalServerError, "failed to fetch partner stats: "+msg)
		return
	}

	response.JSON(w, http.StatusOK, statsResp.Stats)
}

// ListPartnerUsers returns paginated list of users under the current partner
func (h *PartnerHandler) ListPartnerUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "missing or invalid user ID")
		return
	}

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

	var req struct {
		Limit  int32 `json:"limit"`
		Offset int32 `json:"offset"`
	}
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	usersResp, err := h.authClient.PartnerClient.GetUsersByPartnerPaginated(ctx, &authpb.GetUsersByPartnerPaginatedRequest{
		PartnerId: partnerID,
		Limit:     req.Limit,
		Offset:    req.Offset,
	})
	if err != nil || usersResp == nil || !usersResp.Ok {
		msg := "unknown error"
		if err != nil {
			msg = err.Error()
		} else if usersResp != nil {
			msg = usersResp.Error
		}
		response.Error(w, http.StatusInternalServerError, "failed to fetch users: "+msg)
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"users":       usersResp.Users,
		"total_count": usersResp.TotalCount,
		"limit":       req.Limit,
		"offset":      req.Offset,
	})
}

// UpdatePartnerUserStatus allows admin to change user status (active/suspended)
func (h *PartnerHandler) UpdatePartnerUserStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		response.Error(w, http.StatusBadRequest, "missing user id")
		return
	}

	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "missing or invalid user ID")
		return
	}

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

	var req struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	validStatuses := map[string]bool{"active": true, "suspended": true, "inactive": true}
	if !validStatuses[req.Status] {
		response.Error(w, http.StatusBadRequest, "invalid status, must be: active, suspended, or inactive")
		return
	}

	statusResp, err := h.authClient.PartnerClient.UpdateUserStatus(ctx, &authpb.UpdateUserStatusRequest{
		UserId:    targetID,
		PartnerId: partnerID,
		Status:    req.Status,
	})
	if err != nil || statusResp == nil || !statusResp.Ok {
		msg := "unknown error"
		if err != nil {
			msg = err.Error()
		} else if statusResp != nil {
			msg = statusResp.Error
		}
		response.Error(w, http.StatusInternalServerError, "failed to update user status: "+msg)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"user_id": targetID,
		"status":  req.Status,
		"message": "user status updated successfully",
	})
}

// UpdatePartnerUserRole allows admin to change user role (admin/user)
func (h *PartnerHandler) UpdatePartnerUserRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		response.Error(w, http.StatusBadRequest, "missing user id")
		return
	}

	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "missing or invalid user ID")
		return
	}

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

	var req struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	validRoles := map[string]bool{"partner_admin": true, "partner_user": true}
	if !validRoles[req.Role] {
		response.Error(w, http.StatusBadRequest, "invalid role, must be: partner_admin or partner_user")
		return
	}

	roleResp, err := h.authClient.PartnerClient.UpdateUserRole(ctx, &authpb.UpdateUserRoleRequest{
		UserId:    targetID,
		PartnerId: partnerID,
		Role:      req.Role,
	})
	if err != nil || roleResp == nil || !roleResp.Ok {
		msg := "unknown error"
		if err != nil {
			msg = err.Error()
		} else if roleResp != nil {
			msg = roleResp.Error
		}
		response.Error(w, http.StatusInternalServerError, "failed to update user role: "+msg)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"user_id": targetID,
		"role":    req.Role,
		"message": "user role updated successfully",
	})
}

// SearchPartnerUsers searches users within partner by email/phone/name
func (h *PartnerHandler) SearchPartnerUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "missing or invalid user ID")
		return
	}

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

	var req struct {
		SearchTerm string `json:"search_term"`
		Limit      int32  `json:"limit"`
		Offset     int32  `json:"offset"`
	}
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.SearchTerm == "" {
		response.Error(w, http.StatusBadRequest, "search_term is required")
		return
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	searchResp, err := h.authClient.PartnerClient.SearchPartnerUsers(ctx, &authpb.SearchPartnerUsersRequest{
		PartnerId:  partnerID,
		SearchTerm: req.SearchTerm,
		Limit:      req.Limit,
		Offset:     req.Offset,
	})
	if err != nil || searchResp == nil || !searchResp.Ok {
		msg := "unknown error"
		if err != nil {
			msg = err.Error()
		} else if searchResp != nil {
			msg = searchResp.Error
		}
		response.Error(w, http.StatusInternalServerError, "failed to search users: "+msg)
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"users":       searchResp.Users,
		"search_term": req.SearchTerm,
		"limit":       req.Limit,
		"offset":      req.Offset,
	})
}

// GetPartnerUserByEmail retrieves a specific user by email within partner
func (h *PartnerHandler) GetPartnerUserByEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "missing or invalid user ID")
		return
	}

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

	var req struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Email == "" {
		response.Error(w, http.StatusBadRequest, "email is required")
		return
	}

	emailResp, err := h.authClient.PartnerClient.GetPartnerUserByEmail(ctx, &authpb.GetPartnerUserByEmailRequest{
		PartnerId: partnerID,
		Email:     req.Email,
	})
	if err != nil || emailResp == nil || !emailResp.Ok {
		msg := "unknown error"
		if err != nil {
			msg = err.Error()
		} else if emailResp != nil {
			msg = emailResp.Error
		}
		response.Error(w, http.StatusNotFound, "user not found: "+msg)
		return
	}

	response.JSON(w, http.StatusOK, emailResp.User)
}

// BulkUpdateUserStatus allows admin to update status for multiple users at once
func (h *PartnerHandler) BulkUpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "missing or invalid user ID")
		return
	}

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

	var req struct {
		UserIDs []string `json:"user_ids"`
		Status  string   `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.UserIDs) == 0 {
		response.Error(w, http.StatusBadRequest, "user_ids is required and must not be empty")
		return
	}

	validStatuses := map[string]bool{"active": true, "suspended": true, "inactive": true}
	if !validStatuses[req.Status] {
		response.Error(w, http.StatusBadRequest, "invalid status, must be: active, suspended, or inactive")
		return
	}

	bulkResp, err := h.authClient.PartnerClient.BulkUpdateUserStatus(ctx, &authpb.BulkUpdateUserStatusRequest{
		PartnerId: partnerID,
		UserIds:   req.UserIDs,
		Status:    req.Status,
	})
	if err != nil || bulkResp == nil || !bulkResp.Ok {
		msg := "unknown error"
		if err != nil {
			msg = err.Error()
		} else if bulkResp != nil {
			msg = bulkResp.Error
		}
		response.Error(w, http.StatusInternalServerError, "failed to bulk update users: "+msg)
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"updated_count": len(req.UserIDs),
		"status":        req.Status,
		"message":       "users updated successfully",
	})
}