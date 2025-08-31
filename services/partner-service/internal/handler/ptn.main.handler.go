package handler

import (
	//"context"
	"encoding/json"
	"net/http"
	accountclient "x/shared/account"
	emailclient "x/shared/email"
	smsclient "x/shared/sms"

	domain "partner-service/internal/domain"
	"partner-service/internal/usecase"
	authclient "x/shared/auth" // gRPC/HTTP client for auth-service
	otpclient "x/shared/auth/otp"
	"x/shared/response"

	"x/shared/genproto/authpb"
	//"x/shared/genproto/emailpb"
)

type PartnerHandler struct {
	uc         *usecase.PartnerUsecase
	authClient *authclient.AuthService
	otp *otpclient.OTPService
	accountClient *accountclient.AccountClient
	emailClient *emailclient.EmailClient
	smsClient *smsclient.SMSClient
}

func NewPartnerHandler(
	uc *usecase.PartnerUsecase,
	authClient *authclient.AuthService,
	otp          *otpclient.OTPService,
	accountClient *accountclient.AccountClient,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
) *PartnerHandler {
	return &PartnerHandler{
		uc:         uc,
		authClient: authClient,
		otp:            otp,
		accountClient:  accountClient,
		emailClient:    emailClient,
		smsClient:      smsClient,
	}
}

// ----------- Handlers -----------
func decodeJSON(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}
// CreatePartner
func (h *PartnerHandler) CreatePartner(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request (you’d normally have DTOs here)
	var req struct {
		Name         string `json:"name"`
		Country      string `json:"country"`
		ContactEmail string `json:"contact_email"`
		ContactPhone string `json:"contact_phone"`
	}
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest,err.Error())
		return
	}

	partner := &domain.Partner{
		Name:         req.Name,
		Country:      req.Country,
		ContactEmail: req.ContactEmail,
		ContactPhone: req.ContactPhone,
	}

	if err := h.uc.CreatePartner(ctx, partner); err != nil {
		response.Error(w, http.StatusInternalServerError,err.Error(),)
		return
	}

	response.JSON(w, http.StatusCreated, partner)
}

// CreatePartnerUser (calls auth service to create user first)
func (h *PartnerHandler) CreatePartnerUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		PartnerID string `json:"partner_id"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		Role      string `json:"role"` // incoming as plain string
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest,err.Error(),)
		return
	}

	// validate role
	var role domain.PartnerUserRole
	switch req.Role {
	case string(domain.PartnerUserRoleAdmin):
		role = domain.PartnerUserRoleAdmin
	case string(domain.PartnerUserRoleUser):
		role = domain.PartnerUserRoleUser
	default:
		response.Error(w,http.StatusBadRequest, "invalid role, must be 'admin' or 'partner_user'", )
		return
	}

	// Step 1: Create user in auth service
	userResp, err := h.authClient.RegisterUser(ctx, &authpb.RegisterUserRequest{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      string(role), // send plain string to auth
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to create user in auth service: "+err.Error(),)
		return
	}

	// Step 2: Save partner_user
	partnerUser := &domain.PartnerUser{
		PartnerID: req.PartnerID,
		UserID:    userResp.UserId,
		Role:      role, // store as typed enum
		Email:     req.Email,
		IsActive:  true,
	}

	if err := h.uc.CreatePartnerUser(ctx, partnerUser); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, partnerUser)
}

