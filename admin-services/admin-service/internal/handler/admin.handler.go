package handler

import (
	"encoding/json"
	"net/http"

	"x/shared/auth/middleware"
	"x/shared/auth/otp"
	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	coreclient "x/shared/core"
	partnerclient "x/shared/partner"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	"x/shared/response"

	"github.com/redis/go-redis/v9"
)

type AdminHandler struct {
	auth          *middleware.MiddlewareWithClient
	otp           *otpclient.OTPService
	emailClient   *emailclient.EmailClient
	smsClient     *smsclient.SMSClient
	redisClient   *redis.Client
	coreClient    *coreclient.CoreService
	partnerClient *partnerclient.PartnerService
}

func NewAdminHandler(
	auth *middleware.MiddlewareWithClient,
	otp *otpclient.OTPService,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
	redisClient *redis.Client,
	coreClient *coreclient.CoreService,
	partnerClient *partnerclient.PartnerService,
) *AdminHandler {
	return &AdminHandler{
		auth:          auth,
		otp:           otp,
		emailClient:   emailClient,
		smsClient:     smsClient,
		redisClient:   redisClient,
		coreClient:    coreClient,
		partnerClient: partnerClient,
	}
}

// ---------------- Partner Management ----------------

// POST /partners
func (h *AdminHandler) CreatePartner(w http.ResponseWriter, r *http.Request) {
	var req partnersvcpb.CreatePartnerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.partnerClient.Client.CreatePartner(r.Context(), &req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to create partner: "+err.Error())
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

// PUT /partners/{id}
func (h *AdminHandler) UpdatePartner(w http.ResponseWriter, r *http.Request) {
	var req partnersvcpb.UpdatePartnerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.partnerClient.Client.UpdatePartner(r.Context(), &req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to update partner: "+err.Error())
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

// DELETE /partners/{id}
func (h *AdminHandler) DeletePartner(w http.ResponseWriter, r *http.Request) {
	var req partnersvcpb.DeletePartnerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.partnerClient.Client.DeletePartner(r.Context(), &req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to delete partner: "+err.Error())
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

// ---------------- Partner User Management ----------------

// POST /partners/users
func (h *AdminHandler) CreatePartnerUser(w http.ResponseWriter, r *http.Request) {
	var req partnersvcpb.CreatePartnerUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.partnerClient.Client.CreatePartnerUser(r.Context(), &req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to create partner user: "+err.Error())
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

// PUT /partners/users/{id}
func (h *AdminHandler) UpdatePartnerUser(w http.ResponseWriter, r *http.Request) {
	var req partnersvcpb.UpdatePartnerUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.partnerClient.Client.UpdatePartnerUser(r.Context(), &req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to update partner user: "+err.Error())
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

// DELETE /partners/{partnerId}/users
func (h *AdminHandler) DeletePartnerUsers(w http.ResponseWriter, r *http.Request) {
	var req partnersvcpb.DeletePartnerUsersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.partnerClient.Client.DeletePartnerUsers(r.Context(), &req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to delete partner users: "+err.Error())
		return
	}
	response.JSON(w, http.StatusOK, resp)
}
