package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	//"time"

	//"x/shared/auth/middleware"
	accountingpb "x/shared/genproto/shared/accounting/v1"
	"x/shared/response"

	//"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// AGENT MANAGEMENT HANDLERS
// ============================================================================

type CreateAgentDTO struct {
	UserExternalID   *string                `json:"user_external_id,omitempty"`
	Service          *string                `json:"service,omitempty"`
	CommissionRate   *string                `json:"commission_rate,omitempty"`
	RelationshipType string                 `json:"relationship_type"`
	IsActive         bool                   `json:"is_active"`
	Name             *string                `json:"name,omitempty"`
	Metadata         map[string]string      `json:"metadata,omitempty"`
}

type UpdateAgentDTO struct {
	UserExternalID   *string           `json:"user_external_id,omitempty"`
	Service          *string           `json:"service,omitempty"`
	CommissionRate   *string           `json:"commission_rate,omitempty"`
	RelationshipType *string           `json:"relationship_type,omitempty"`
	IsActive         *bool             `json:"is_active,omitempty"`
	Name             *string           `json:"name,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// Helper function to map relationship type
func mapRelationshipType(s string) accountingpb. RelationshipType {
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

// POST /api/admin/agents
func (h *AdminHandler) CreateAgent(w http. ResponseWriter, r *http.Request) {
	var dto CreateAgentDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if dto. RelationshipType == "" {
		dto.RelationshipType = "direct" // default
	}

	req := &accountingpb.CreateAgentRequest{
		UserExternalId:   dto.UserExternalID,
		Service:          dto.Service,
		CommissionRate:   dto.CommissionRate,
		RelationshipType: mapRelationshipType(dto.RelationshipType),
		IsActive:         dto.IsActive,
		Name:             dto.Name,
		Metadata:         dto.Metadata,
	}

	resp, err := h.accountingClient. Client.CreateAgent(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to create agent: "+err.Error())
		return
	}

	response. JSON(w, http.StatusCreated, resp)
}

// PUT /api/admin/agents/{agent_id}
func (h *AdminHandler) UpdateAgent(w http.ResponseWriter, r *http. Request) {
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
		AgentExternalId: agentID,
		UserExternalId:  dto.UserExternalID,
		Service:         dto.Service,
		CommissionRate:  dto.CommissionRate,
		IsActive:        dto.IsActive,
		Name:            dto.Name,
		Metadata:        dto.Metadata,
	}

	// Convert relationship type if provided
	if dto.RelationshipType != nil {
		relType := mapRelationshipType(*dto.RelationshipType)
		req.RelationshipType = &relType
	}

	resp, err := h.accountingClient.Client.UpdateAgent(r.Context(), req)
	if err != nil {
		response. Error(w, http.StatusBadGateway, "failed to update agent: "+err.Error())
		return
	}

	response.JSON(w, http. StatusOK, resp)
}

// DELETE /api/admin/agents/{agent_id}
func (h *AdminHandler) DeleteAgent(w http.ResponseWriter, r *http. Request) {
	agentID := r.PathValue("agent_id")
	if agentID == "" {
		response.Error(w, http.StatusBadRequest, "agent_id required")
		return
	}

	req := &accountingpb. DeleteAgentRequest{
		AgentExternalId: agentID,
	}

	resp, err := h.accountingClient. Client.DeleteAgent(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to delete agent: "+err.Error())
		return
	}

	response. JSON(w, http.StatusOK, resp)
}

// GET /api/admin/agents/{agent_id}
func (h *AdminHandler) GetAgentByID(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agent_id")
	if agentID == "" {
		response.Error(w, http.StatusBadRequest, "agent_id required")
		return
	}

	req := &accountingpb.GetAgentByIDRequest{
		AgentExternalId: agentID,
	}

	resp, err := h.accountingClient.Client. GetAgentByID(r. Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get agent: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GET /api/admin/agents/user/{user_id}
func (h *AdminHandler) GetAgentByUserID(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	if userID == "" {
		response.Error(w, http. StatusBadRequest, "user_id required")
		return
	}

	req := &accountingpb.GetAgentByUserIDRequest{
		UserExternalId: userID,
	}

	resp, err := h. accountingClient.Client.GetAgentByUserID(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get agent: "+err.Error())
		return
	}

	response. JSON(w, http.StatusOK, resp)
}

// GET /api/admin/agents? limit=20&offset=0
func (h *AdminHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query(). Get("limit")
	offsetStr := r.URL.Query(). Get("offset")

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
		Limit:  limit,
		Offset: offset,
	}

	resp, err := h.accountingClient.Client.ListAgents(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to list agents: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GET /api/admin/agents/{agent_id}/commissions? limit=20&offset=0
func (h *AdminHandler) ListCommissionsForAgent(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agent_id")
	if agentID == "" {
		response.Error(w, http. StatusBadRequest, "agent_id required")
		return
	}

	limitStr := r. URL.Query().Get("limit")
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
		response.Error(w, http.StatusBadGateway, "failed to list commissions: "+err.Error())
		return
	}

	response. JSON(w, http.StatusOK, resp)
}