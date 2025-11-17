package handler

import (
	"encoding/json"
	"net/http"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	"x/shared/response"

)

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
