package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"errors"
	
	partnersvcpb "x/shared/genproto/partner/svcpb"
	"x/shared/response"
	"go.uber.org/zap"
)

// ---------------- Partner Management ----------------

// POST /partners
func (h *AdminHandler) CreatePartner(w http.ResponseWriter, r *http.Request) {
	var req partnersvcpb. CreatePartnerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("failed to decode create partner request", zap.Error(err))
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// ✅ Validate required fields
	if err := validateCreatePartnerRequest(&req); err != nil {
		h.logger. Warn("invalid create partner request", zap.Error(err))
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// ✅ Normalize email
	req.ContactEmail = strings.ToLower(strings.TrimSpace(req.ContactEmail))
	req.LocalCurrency = strings.ToUpper(strings.TrimSpace(req.LocalCurrency))
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))

	resp, err := h.partnerClient.Client.CreatePartner(r.Context(), &req)
	if err != nil {
		h.logger.Error("failed to create partner",
			zap.String("name", req.Name),
			zap.String("service", req.Service),
			zap.Error(err))
		response.Error(w, http.StatusInternalServerError, "failed to create partner: "+err.Error())
		return
	}

	h.logger.Info("partner created successfully",
		zap.String("partner_id", resp.Partner.Id),
		zap.String("name", resp.Partner.Name),
		zap.String("service", resp. Partner.Service),
		zap.String("currency", resp.Partner.Currency),
		zap.Float64("rate", resp.Partner.Rate))

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message":  "Partner created successfully",
		"data":     resp.Partner,
		// ✅ Include API credentials if available (send via secure channel in production)
		"api_key":  resp.Partner.ApiKey,
	})
}

// PUT /partners/{id}
func (h *AdminHandler) UpdatePartner(w http.ResponseWriter, r *http.Request) {
	var req partnersvcpb.UpdatePartnerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger. Error("failed to decode update partner request", zap.Error(err))
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// ✅ Validate ID
	if req.Id == "" {
		response.Error(w, http.StatusBadRequest, "partner id is required")
		return
	}

	// ✅ Validate update fields if provided
	if err := validateUpdatePartnerRequest(&req); err != nil {
		h.logger.Warn("invalid update partner request",
			zap.String("partner_id", req.Id),
			zap.Error(err))
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// ✅ Normalize fields
	if req.ContactEmail != "" {
		req. ContactEmail = strings.ToLower(strings.TrimSpace(req.ContactEmail))
	}
	if req.LocalCurrency != "" {
		req.LocalCurrency = strings.ToUpper(strings.TrimSpace(req. LocalCurrency))
	}
	if req.Currency != "" {
		req.Currency = strings. ToUpper(strings.TrimSpace(req.Currency))
	}

	resp, err := h.partnerClient.Client.UpdatePartner(r.Context(), &req)
	if err != nil {
		h.logger.Error("failed to update partner",
			zap.String("partner_id", req.Id),
			zap.Error(err))
		response.Error(w, http.StatusInternalServerError, "failed to update partner: "+err.Error())
		return
	}

	h. logger.Info("partner updated successfully",
		zap.String("partner_id", resp.Partner.Id),
		zap.String("name", resp.Partner.Name))

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Partner updated successfully",
		"data":    resp.Partner,
	})
}

// DELETE /partners/{id}
func (h *AdminHandler) DeletePartner(w http.ResponseWriter, r *http.Request) {
	var req partnersvcpb.DeletePartnerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger. Error("failed to decode delete partner request", zap.Error(err))
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// ✅ Validate ID
	if req.Id == "" {
		response.Error(w, http.StatusBadRequest, "partner id is required")
		return
	}

	resp, err := h.partnerClient.Client.DeletePartner(r.Context(), &req)
	if err != nil {
		h.logger.Error("failed to delete partner",
			zap.String("partner_id", req.Id),
			zap.Error(err))
		response.Error(w, http.StatusInternalServerError, "failed to delete partner:  "+err.Error())
		return
	}

	if ! resp.Success {
		response.Error(w, http.StatusInternalServerError, "failed to delete partner")
		return
	}

	h.logger.Info("partner deleted successfully",
		zap.String("partner_id", req.Id))

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Partner deleted successfully",
	})
}

// ✅ NEW: GET /partners
func (h *AdminHandler) GetPartners(w http.ResponseWriter, r *http.Request) {
	var req partnersvcpb.GetPartnersRequest
	
	// Optional:  Parse partner IDs from query params
	// Example: /partners?ids=PTN123,PTN456
	if idsParam := r.URL.Query().Get("ids"); idsParam != "" {
		req.PartnerIds = strings.Split(idsParam, ",")
	}

	resp, err := h.partnerClient.Client.GetPartners(r.Context(), &req)
	if err != nil {
		h.logger.Error("failed to get partners", zap. Error(err))
		response.Error(w, http.StatusInternalServerError, "failed to get partners: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    resp.Partners,
		"count":   len(resp. Partners),
	})
}

// ✅ NEW: GET /partners/service/{service}
func (h *AdminHandler) GetPartnersByService(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	if service == "" {
		response. Error(w, http.StatusBadRequest, "service parameter is required")
		return
	}

	resp, err := h.partnerClient.Client.GetPartnersByService(r.Context(), &partnersvcpb.GetPartnersByServiceRequest{
		Service: service,
	})
	if err != nil {
		h.logger.Error("failed to get partners by service",
			zap.String("service", service),
			zap.Error(err))
		response.Error(w, http.StatusInternalServerError, "failed to get partners:  "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    resp.Partners,
		"count":   len(resp.Partners),
		"service": service,
	})
}

// ---------------- Partner User Management ----------------

// POST /partners/users
func (h *AdminHandler) CreatePartnerUser(w http.ResponseWriter, r *http.Request) {
	var req partnersvcpb.CreatePartnerUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("failed to decode create partner user request", zap.Error(err))
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// ✅ Validate required fields
	if req.PartnerId == "" {
		response.Error(w, http.StatusBadRequest, "partner_id is required")
		return
	}
	if req.Email == "" && req.Phone == "" {
		response.Error(w, http.StatusBadRequest, "email or phone is required")
		return
	}
	if req.FirstName == "" || req.LastName == "" {
		response.Error(w, http.StatusBadRequest, "first_name and last_name are required")
		return
	}
	if req.Role == "" {
		req.Role = "partner_user" // Default role
	}

	// ✅ Normalize email
	if req.Email != "" {
		req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	}

	resp, err := h.partnerClient.Client.CreatePartnerUser(r.Context(), &req)
	if err != nil {
		h.logger.Error("failed to create partner user",
			zap.String("partner_id", req.PartnerId),
			zap.String("email", req.Email),
			zap.Error(err))
		response.Error(w, http.StatusInternalServerError, "failed to create partner user: "+err.Error())
		return
	}

	h.logger.Info("partner user created successfully",
		zap.String("user_id", resp.User.Id),
		zap.String("partner_id", req.PartnerId),
		zap.String("email", req.Email))

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message":  "Partner user created successfully",
		"data":    resp.User,
	})
}

// PUT /partners/users/{id}
func (h *AdminHandler) UpdatePartnerUser(w http.ResponseWriter, r *http.Request) {
	var req partnersvcpb.UpdatePartnerUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("failed to decode update partner user request", zap.Error(err))
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// ✅ Validate ID
	if req. Id == "" {
		response. Error(w, http.StatusBadRequest, "user id is required")
		return
	}

	// ✅ Normalize email
	if req.Email != "" {
		req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	}

	resp, err := h.partnerClient.Client. UpdatePartnerUser(r.Context(), &req)
	if err != nil {
		h.logger.Error("failed to update partner user",
			zap.String("user_id", req.Id),
			zap.Error(err))
		response.Error(w, http.StatusInternalServerError, "failed to update partner user: "+err.Error())
		return
	}

	h.logger.Info("partner user updated successfully",
		zap.String("user_id", resp.User.Id))

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Partner user updated successfully",
		"data":    resp.User,
	})
}

// DELETE /partners/{partnerId}/users
func (h *AdminHandler) DeletePartnerUsers(w http.ResponseWriter, r *http.Request) {
	var req partnersvcpb. DeletePartnerUsersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("failed to decode delete partner users request", zap.Error(err))
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// ✅ Validate required fields
	if req.PartnerId == "" {
		response. Error(w, http.StatusBadRequest, "partner_id is required")
		return
	}
	if len(req.UserIds) == 0 {
		response.Error(w, http.StatusBadRequest, "user_ids are required")
		return
	}

	resp, err := h.partnerClient.Client.DeletePartnerUsers(r. Context(), &req)
	if err != nil {
		h. logger.Error("failed to delete partner users",
			zap.String("partner_id", req.PartnerId),
			zap.Strings("user_ids", req. UserIds),
			zap.Error(err))
		response.Error(w, http.StatusInternalServerError, "failed to delete partner users: "+err.Error())
		return
	}

	h.logger.Info("partner users deleted",
		zap.String("partner_id", req.PartnerId),
		zap.Int("deleted_count", len(resp.DeletedIds)),
		zap.Int("failed_count", len(resp. FailedUsers)))

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"message":      "Partner users deletion completed",
		"deleted_ids":   resp.DeletedIds,
		"failed_users": resp.FailedUsers,
		"deleted_count": len(resp.DeletedIds),
		"failed_count":  len(resp.FailedUsers),
	})
}

// ✅ NEW: GET /partners/{partnerId}/users
func (h *AdminHandler) GetPartnerUsers(w http.ResponseWriter, r *http.Request) {
	partnerID := r.URL.Query().Get("partner_id")
	if partnerID == "" {
		response. Error(w, http.StatusBadRequest, "partner_id is required")
		return
	}

	resp, err := h.partnerClient.Client.GetPartnerUsers(r.Context(), &partnersvcpb.GetPartnerUsersRequest{
		PartnerId:  partnerID,
	})
	if err != nil {
		h.logger.Error("failed to get partner users",
			zap.String("partner_id", partnerID),
			zap.Error(err))
		response.Error(w, http.StatusInternalServerError, "failed to get partner users: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    resp.Users,
		"count":   len(resp.Users),
	})
}

// ---------------- Validation Helpers ----------------

// validateCreatePartnerRequest validates partner creation request
func validateCreatePartnerRequest(req *partnersvcpb.CreatePartnerRequest) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	if req.Service == "" {
		return errors.New("service is required")
	}
	if req.Currency == "" {
		return errors.New("currency is required")
	}
	if req.LocalCurrency == "" {
		return errors.New("local_currency is required (e.g., KES, USD)")
	}
	if req.Rate <= 0 {
		return errors.New("rate must be greater than 0")
	}
	if len(req.LocalCurrency) != 3 {
		return errors.New("local_currency must be a 3-letter code")
	}
	if req.CommissionRate < 0 || req.CommissionRate > 1 {
		return errors.New("commission_rate must be between 0 and 1")
	}
	if req.ContactEmail == "" && req.ContactPhone == "" {
		return errors.New("at least one contact method (email or phone) is required")
	}
	return nil
}

// validateUpdatePartnerRequest validates partner update request
func validateUpdatePartnerRequest(req *partnersvcpb.UpdatePartnerRequest) error {
	if req.LocalCurrency != "" && len(req.LocalCurrency) != 3 {
		return errors.New("local_currency must be a 3-letter code")
	}
	if req. Rate < 0 {
		return errors.New("rate cannot be negative")
	}
	if req.CommissionRate < 0 || req.CommissionRate > 1 {
		return errors.New("commission_rate must be between 0 and 1")
	}
	return nil
}