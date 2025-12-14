package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	accountingpb "x/shared/genproto/shared/accounting/v1"
	"x/shared/response"
)

// ============================================================================
// AGENT MANAGEMENT HANDLERS
// ============================================================================

type CreateAgentDTO struct {
	UserExternalID           *string           `json:"user_external_id,omitempty"`
	Service                  *string           `json:"service,omitempty"`
	CommissionRate           *string           `json:"commission_rate,omitempty"`
	RelationshipType         string            `json:"relationship_type"`
	IsActive                 bool              `json:"is_active"`
	Name                     *string           `json:"name,omitempty"`
	Metadata                 map[string]string `json:"metadata,omitempty"`
	
	// ✅ NEW FIELDS
	CommissionRateForDeposit *string         `json:"commission_rate_for_deposit,omitempty"`
	PaymentMethod            *string         `json:"payment_method,omitempty"`
	Location                 map[string]bool `json:"location,omitempty"` // {"KE": true, "UG": false}
	Status                   *string         `json:"status,omitempty"`   // "active", "inactive", "deleted"
}

type UpdateAgentDTO struct {
	UserExternalID           *string           `json:"user_external_id,omitempty"`
	Service                  *string           `json:"service,omitempty"`
	CommissionRate           *string           `json:"commission_rate,omitempty"`
	RelationshipType         *string           `json:"relationship_type,omitempty"`
	IsActive                 *bool             `json:"is_active,omitempty"`
	Name                     *string           `json:"name,omitempty"`
	Metadata                 map[string]string `json:"metadata,omitempty"`
	
	// ✅ NEW FIELDS
	CommissionRateForDeposit *string         `json:"commission_rate_for_deposit,omitempty"`
	PaymentMethod            *string         `json:"payment_method,omitempty"`
	Location                 map[string]bool `json:"location,omitempty"`
	Status                   *string         `json:"status,omitempty"`
}

// Helper function to map relationship type
func mapRelationshipType(s string) accountingpb.RelationshipType {
	switch s {
	case "direct": 
		return accountingpb.RelationshipType_RELATIONSHIP_TYPE_DIRECT
	case "referral":
		return accountingpb.RelationshipType_RELATIONSHIP_TYPE_REFERRAL
	case "partner":
		return accountingpb.RelationshipType_RELATIONSHIP_TYPE_PARTNER
	case "franchise": 
		return accountingpb.RelationshipType_RELATIONSHIP_TYPE_FRANCHISE
	default: 
		return accountingpb.RelationshipType_RELATIONSHIP_TYPE_UNSPECIFIED
	}
}

// ✅ NEW:  Helper function to map agent status
func mapAgentStatus(s string) accountingpb.AgentStatus {
	switch s {
	case "active":
		return accountingpb.AgentStatus_AGENT_STATUS_ACTIVE
	case "inactive":
		return accountingpb.AgentStatus_AGENT_STATUS_INACTIVE
	case "deleted":
		return accountingpb.AgentStatus_AGENT_STATUS_DELETED
	default:
		return accountingpb.AgentStatus_AGENT_STATUS_ACTIVE // default to active
	}
}

// POST /api/admin/agents
func (h *AdminHandler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	var dto CreateAgentDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if dto.RelationshipType == "" {
		dto.RelationshipType = "direct" // default
	}

	req := &accountingpb.CreateAgentRequest{
		UserExternalId:           dto.UserExternalID,
		Service:                  dto.Service,
		CommissionRate:           dto.CommissionRate,
		RelationshipType:         mapRelationshipType(dto.RelationshipType),
		IsActive:                 dto.IsActive,
		Name:                     dto.Name,
		Metadata:                 dto.Metadata,
		CommissionRateForDeposit:  dto.CommissionRateForDeposit, // ✅ NEW
		PaymentMethod:            dto.PaymentMethod,            // ✅ NEW
		Location:                 dto.Location,                 // ✅ NEW
	}

	// ✅ Convert status if provided
	if dto.Status != nil {
		status := mapAgentStatus(*dto.Status)
		req.Status = &status
	}

	resp, err := h.accountingClient.Client.CreateAgent(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to create agent:  "+err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, resp)
}

// PUT /api/admin/agents/{agent_id}
func (h *AdminHandler) UpdateAgent(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agent_id")
	if agentID == "" {
		response.Error(w, http.StatusBadRequest, "agent_id required")
		return
	}

	var dto UpdateAgentDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req := &accountingpb.UpdateAgentRequest{
		AgentExternalId:           agentID,
		UserExternalId:           dto.UserExternalID,
		Service:                  dto.Service,
		CommissionRate:           dto.CommissionRate,
		IsActive:                 dto.IsActive,
		Name:                     dto.Name,
		Metadata:                 dto.Metadata,
		CommissionRateForDeposit: dto.CommissionRateForDeposit, // ✅ NEW
		PaymentMethod:            dto.PaymentMethod,            // ✅ NEW
		Location:                  dto.Location,                 // ✅ NEW
	}

	// Convert relationship type if provided
	if dto.RelationshipType != nil {
		relType := mapRelationshipType(*dto.RelationshipType)
		req.RelationshipType = &relType
	}

	// ✅ Convert status if provided
	if dto.Status != nil {
		status := mapAgentStatus(*dto.Status)
		req.Status = &status
	}

	resp, err := h.accountingClient.Client.UpdateAgent(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to update agent: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// DELETE /api/admin/agents/{agent_id}
func (h *AdminHandler) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agent_id")
	if agentID == "" {
		response.Error(w, http.StatusBadRequest, "agent_id required")
		return
	}

	req := &accountingpb.DeleteAgentRequest{
		AgentExternalId: agentID,
	}

	resp, err := h.accountingClient.Client.DeleteAgent(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to delete agent: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GET /api/admin/agents/{agent_id}? include_accounts=true
func (h *AdminHandler) GetAgentByID(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agent_id")
	if agentID == "" {
		response.Error(w, http.StatusBadRequest, "agent_id required")
		return
	}

	// ✅ Parse include_accounts query param
	includeAccounts := r.URL.Query().Get("include_accounts") == "true"

	req := &accountingpb.GetAgentByIDRequest{
		AgentExternalId: agentID,
		IncludeAccounts: includeAccounts, // ✅ NEW
	}

	resp, err := h.accountingClient.Client.GetAgentByID(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get agent: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GET /api/admin/agents/user/{user_id}?include_accounts=true
func (h *AdminHandler) GetAgentByUserID(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	if userID == "" {
		response.Error(w, http.StatusBadRequest, "user_id required")
		return
	}

	// ✅ Parse include_accounts query param
	includeAccounts := r.URL.Query().Get("include_accounts") == "true"

	req := &accountingpb.GetAgentByUserIDRequest{
		UserExternalId:   userID,
		IncludeAccounts: includeAccounts, // ✅ NEW
	}

	resp, err := h.accountingClient.Client.GetAgentByUserID(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get agent: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// ✅ UPDATED: GET /api/admin/agents? limit=20&offset=0&country=KE&payment_method=mpesa&status=active&include_accounts=true
func (h *AdminHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	
	// ✅ NEW: Filter params
	countryCode := r.URL.Query().Get("country")
	paymentMethod := r.URL.Query().Get("payment_method")
	statusStr := r.URL.Query().Get("status")
	relationshipTypeStr := r.URL.Query().Get("relationship_type")
	includeAccounts := r.URL.Query().Get("include_accounts") == "true"

	limit := int32(20) // default
	offset := int32(0)

	if limitStr != "" {
		if l, err := strconv.ParseInt(limitStr, 10, 32); err == nil {
			limit = int32(l)
		}
	}
	if offsetStr != "" {
		if o, err := strconv.ParseInt(offsetStr, 10, 32); err == nil {
			offset = int32(o)
		}
	}

	req := &accountingpb.ListAgentsRequest{
		Limit:           limit,
		Offset:          offset,
		IncludeAccounts: includeAccounts, // ✅ NEW
	}

	// ✅ Add filters if provided
	if countryCode != "" {
		req.CountryCode = &countryCode
	}
	if paymentMethod != "" {
		req.PaymentMethod = &paymentMethod
	}
	if statusStr != "" {
		status := mapAgentStatus(statusStr)
		req.Status = &status
	}
	if relationshipTypeStr != "" {
		relType := mapRelationshipType(relationshipTypeStr)
		req.RelationshipType = &relType
	}

	resp, err := h.accountingClient.Client.ListAgents(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to list agents: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// ✅ NEW: GET /api/admin/agents/by-countries?countries=KE,UG,TZ&limit=50&status=active&include_accounts=true
func (h *AdminHandler) GetAgentsByCountries(w http.ResponseWriter, r *http.Request) {
	countriesStr := r.URL.Query().Get("countries")
	if countriesStr == "" {
		response.Error(w, http.StatusBadRequest, "countries parameter required (comma-separated)")
		return
	}

	countries := strings.Split(countriesStr, ",")
	for i := range countries {
		countries[i] = strings.TrimSpace(countries[i])
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	statusStr := r.URL.Query().Get("status")
	includeAccounts := r.URL.Query().Get("include_accounts") == "true"

	limit := int32(50) // default
	offset := int32(0)

	if limitStr != "" {
		if l, err := strconv.ParseInt(limitStr, 10, 32); err == nil {
			limit = int32(l)
		}
	}
	if offsetStr != "" {
		if o, err := strconv.ParseInt(offsetStr, 10, 32); err == nil {
			offset = int32(o)
		}
	}

	req := &accountingpb.GetAgentsByCountriesRequest{
		CountryCodes:     countries,
		Limit:           limit,
		Offset:          offset,
		IncludeAccounts: includeAccounts,
	}

	if statusStr != "" {
		status := mapAgentStatus(statusStr)
		req.Status = &status
	}

	resp, err := h.accountingClient.Client.GetAgentsByCountries(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get agents by countries: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// ✅ NEW: GET /api/admin/agents/stats? country=KE&payment_method=mpesa
func (h *AdminHandler) GetAgentStats(w http.ResponseWriter, r *http.Request) {
	countryCode := r.URL.Query().Get("country")
	paymentMethod := r.URL.Query().Get("payment_method")

	req := &accountingpb.GetAgentStatsRequest{}

	if countryCode != "" {
		req.CountryCode = &countryCode
	}
	if paymentMethod != "" {
		req.PaymentMethod = &paymentMethod
	}

	resp, err := h.accountingClient.Client.GetAgentStats(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get agent stats: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GET /api/admin/agents/{agent_id}/commissions? limit=20&offset=0
func (h *AdminHandler) ListCommissionsForAgent(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agent_id")
	if agentID == "" {
		response.Error(w, http.StatusBadRequest, "agent_id required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := int32(20) // default
	offset := int32(0)

	if limitStr != "" {
		if l, err := strconv.ParseInt(limitStr, 10, 32); err == nil {
			limit = int32(l)
		}
	}
	if offsetStr != "" {
		if o, err := strconv.ParseInt(offsetStr, 10, 32); err == nil {
			offset = int32(o)
		}
	}

	req := &accountingpb.ListCommissionsForAgentRequest{
		AgentExternalId: agentID,
		Limit:           limit,
		Offset:          offset,
	}

	resp, err := h.accountingClient.Client.ListCommissionsForAgent(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to list commissions:  "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}