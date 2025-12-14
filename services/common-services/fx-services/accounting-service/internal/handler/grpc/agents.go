package hgrpc

import (
	"context"
	"fmt"

	"accounting-service/internal/domain"
	"accounting-service/internal/usecase"
	accountingpb "x/shared/genproto/shared/accounting/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ===============================
// AGENT MANAGEMENT
// ===============================

func (h *AccountingHandler) CreateAgent(
	ctx context.Context,
	req *accountingpb.CreateAgentRequest,
) (*accountingpb.CreateAgentResponse, error) {
	// Validate
	if req.RelationshipType == accountingpb.RelationshipType_RELATIONSHIP_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "relationship_type is required")
	}

	// Convert metadata from map<string,string> to map[string]interface{}
	metadata := make(map[string]interface{})
	for k, v := range req. Metadata {
		metadata[k] = v
	}

	// Convert to usecase request
	createReq := usecase.CreateAgentRequest{
		UserExternalID:           req.UserExternalId,
		Service:                  req.Service,
		CommissionRate:           req.CommissionRate,
		RelationshipType:         convertRelationshipTypeToDomain(req.RelationshipType),
		IsActive:                 req.IsActive,
		Name:                     req.Name,
		Metadata:                 metadata,
		CommissionRateForDeposit:  req.CommissionRateForDeposit, // ✅ NEW
		PaymentMethod:            req.PaymentMethod,            // ✅ NEW
		Location:                 req.Location,                 // ✅ NEW
	}

	// ✅ Convert status if provided
	if req.Status != nil {
		statusStr := convertAgentStatusToDomain(*req. Status)
		createReq.Status = &statusStr
	}

	// Execute
	agent, err := h.agentUC.CreateAgent(ctx, createReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.CreateAgentResponse{
		Agent:   convertAgentToProto(agent),
		Message: "Agent created successfully",
	}, nil
}

func (h *AccountingHandler) UpdateAgent(
	ctx context.Context,
	req *accountingpb. UpdateAgentRequest,
) (*accountingpb.UpdateAgentResponse, error) {
	// Validate
	if req.AgentExternalId == "" {
		return nil, status. Error(codes.InvalidArgument, "agent_external_id is required")
	}

	// Convert metadata
	var metadata map[string]interface{}
	if len(req.Metadata) > 0 {
		metadata = make(map[string]interface{})
		for k, v := range req.Metadata {
			metadata[k] = v
		}
	}

	// Convert to usecase request
	updateReq := usecase.UpdateAgentRequest{
		UserExternalID:            req.UserExternalId,
		Service:                  req. Service,
		CommissionRate:            req.CommissionRate,
		IsActive:                 convertOptionalBool(req.IsActive),
		Name:                     req.Name,
		Metadata:                 metadata,
		CommissionRateForDeposit: req.CommissionRateForDeposit, // ✅ NEW
		PaymentMethod:            req.PaymentMethod,            // ✅ NEW
		Location:                  req.Location,                 // ✅ NEW
	}

	// Convert relationship type if provided
	if req.RelationshipType != nil {
		relType := convertRelationshipTypeToDomain(*req.RelationshipType)
		updateReq.RelationshipType = &relType
	}

	// ✅ Convert status if provided
	if req.Status != nil {
		statusStr := convertAgentStatusToDomain(*req.Status)
		updateReq.Status = &statusStr
	}

	// Execute
	agent, err := h.agentUC. UpdateAgent(ctx, req.AgentExternalId, updateReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.UpdateAgentResponse{
		Agent:   convertAgentToProto(agent),
		Message: "Agent updated successfully",
	}, nil
}

func (h *AccountingHandler) DeleteAgent(
	ctx context.Context,
	req *accountingpb.DeleteAgentRequest,
) (*accountingpb.DeleteAgentResponse, error) {
	// Validate
	if req.AgentExternalId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_external_id is required")
	}

	// Execute (soft delete)
	if err := h.agentUC.DeleteAgent(ctx, req.AgentExternalId); err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.DeleteAgentResponse{
		Message: "Agent deleted successfully",
	}, nil
}

func (h *AccountingHandler) GetAgentByID(
	ctx context.Context,
	req *accountingpb.GetAgentByIDRequest,
) (*accountingpb.GetAgentByIDResponse, error) {
	// Validate
	if req.AgentExternalId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_external_id is required")
	}

	// Pass includeAccounts flag to usecase
	agent, err := h. agentUC.GetAgentByID(ctx, req.AgentExternalId, req. IncludeAccounts)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetAgentByIDResponse{
		Agent: convertAgentToProto(agent),
	}, nil
}

func (h *AccountingHandler) GetAgentByUserID(
	ctx context.Context,
	req *accountingpb. GetAgentByUserIDRequest,
) (*accountingpb.GetAgentByUserIDResponse, error) {
	// Validate
	if req.UserExternalId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_external_id is required")
	}

	// Pass includeAccounts flag to usecase
	agent, err := h.agentUC. GetAgentByUserID(ctx, req.UserExternalId, req.IncludeAccounts)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetAgentByUserIDResponse{
		Agent: convertAgentToProto(agent),
	}, nil
}

// ✅ UPDATED: ListAgents with filtering support
func (h *AccountingHandler) ListAgents(
	ctx context.Context,
	req *accountingpb.ListAgentsRequest,
) (*accountingpb.ListAgentsResponse, error) {
	// Set defaults
	limit := int(req. Limit)
	if limit == 0 {
		limit = 20
	}
	offset := int(req.Offset)

	var agents []*domain.Agent
	var err error

	// ✅ Check if filters are provided
	hasFilters := req.CountryCode != nil || req.PaymentMethod != nil || req.Status != nil || req.RelationshipType != nil

	if hasFilters {
		// Use filtered query
		filters := usecase.AgentFilters{
			CountryCode:      req.CountryCode,
			PaymentMethod:    req.PaymentMethod,
			RelationshipType: convertOptionalRelationshipTypeToDomain(req.RelationshipType),
			Limit:            limit,
			Offset:           offset,
		}

		// Convert status if provided
		if req.Status != nil {
			statusStr := convertAgentStatusToDomain(*req.Status)
			filters.Status = &statusStr
		}

		agents, err = h. agentUC.ListAgentsWithFilters(ctx, filters, req. IncludeAccounts)
	} else {
		// Use default list (no filters)
		agents, err = h.agentUC. ListAgents(ctx, limit, offset, req.IncludeAccounts)
	}

	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.ListAgentsResponse{
		Agents:  convertAgentsToProto(agents),
		Total:  int32(len(agents)),
	}, nil
}

// ✅ NEW: GetAgentsByCountries
func (h *AccountingHandler) GetAgentsByCountries(
	ctx context.Context,
	req *accountingpb.GetAgentsByCountriesRequest,
) (*accountingpb.GetAgentsByCountriesResponse, error) {
	// Validate
	if len(req.CountryCodes) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one country code is required")
	}

	// Set defaults
	limit := int(req. Limit)
	if limit == 0 {
		limit = 50
	}
	offset := int(req.Offset)

	// Query each country and aggregate results
	allAgents := make([]*domain. Agent, 0)
	seenAgents := make(map[string]bool) // Deduplicate agents that match multiple countries

	for _, countryCode := range req.CountryCodes {
		agents, err := h.agentUC.ListAgentsByCountry(ctx, countryCode, limit, offset, req.IncludeAccounts)
		if err != nil {
			return nil, handleUsecaseError(err)
		}

		// Deduplicate
		for _, agent := range agents {
			if !seenAgents[agent.AgentExternalID] {
				allAgents = append(allAgents, agent)
				seenAgents[agent.AgentExternalID] = true
			}
		}
	}

	// Apply status filter if provided
	if req. Status != nil {
		filteredAgents := make([]*domain.Agent, 0)
		targetStatus := convertAgentStatusToDomain(*req.Status)
		for _, agent := range allAgents {
			if string(agent.Status) == targetStatus {
				filteredAgents = append(filteredAgents, agent)
			}
		}
		allAgents = filteredAgents
	}

	return &accountingpb.GetAgentsByCountriesResponse{
		Agents: convertAgentsToProto(allAgents),
		Total:  int32(len(allAgents)),
	}, nil
}

// ✅ NEW: GetAgentStats
func (h *AccountingHandler) GetAgentStats(
	ctx context.Context,
	req *accountingpb.GetAgentStatsRequest,
) (*accountingpb.GetAgentStatsResponse, error) {
	// For now, we'll implement basic stats by querying agents
	// In production, you might want to add dedicated stats queries to the repository

	// Get all agents (or filtered by country/payment method if provided)
	filters := usecase.AgentFilters{
		CountryCode:   req.CountryCode,
		PaymentMethod: req.PaymentMethod,
		Limit:         1000, // Get a large sample
		Offset:        0,
	}

	agents, err := h.agentUC.ListAgentsWithFilters(ctx, filters, false)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	// Calculate statistics
	totalAgents := int32(len(agents))
	activeAgents := int32(0)
	inactiveAgents := int32(0)
	agentsByCountry := make(map[string]int32)
	agentsByPaymentMethod := make(map[string]int32)

	for _, agent := range agents {
		// Count by status
		if agent.Status == domain.AgentStatusActive {
			activeAgents++
		} else if agent.Status == domain.AgentStatusInactive {
			inactiveAgents++
		}

		// Count by country
		if agent.Location != nil {
			for country, enabled := range agent.Location {
				if enabled {
					agentsByCountry[country]++
				}
			}
		}

		// Count by payment method
		if agent.PaymentMethod != nil {
			agentsByPaymentMethod[*agent.PaymentMethod]++
		}
	}

	return &accountingpb.GetAgentStatsResponse{
		TotalAgents:           totalAgents,
		ActiveAgents:          activeAgents,
		InactiveAgents:        inactiveAgents,
		AgentsByCountry:       agentsByCountry,
		AgentsByPaymentMethod: agentsByPaymentMethod,
	}, nil
}

func (h *AccountingHandler) ListCommissionsForAgent(
	ctx context.Context,
	req *accountingpb.ListCommissionsForAgentRequest,
) (*accountingpb.ListCommissionsForAgentResponse, error) {
	// Validate
	if req.AgentExternalId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_external_id is required")
	}

	// Set defaults
	limit := int(req.Limit)
	if limit == 0 {
		limit = 20
	}
	offset := int(req.Offset)

	// Execute
	commissions, err := h.agentUC.ListCommissionsForAgent(ctx, req.AgentExternalId, limit, offset)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.ListCommissionsForAgentResponse{
		Commissions: convertAgentCommissionsToProto(commissions),
		Total:       int32(len(commissions)),
	}, nil
}

// ===============================
// CONVERSION HELPERS
// ===============================

func convertRelationshipTypeToDomain(rt accountingpb.RelationshipType) string {
	switch rt {
	case accountingpb.RelationshipType_RELATIONSHIP_TYPE_DIRECT:
		return "direct"
	case accountingpb.RelationshipType_RELATIONSHIP_TYPE_REFERRAL:
		return "referral"
	case accountingpb.RelationshipType_RELATIONSHIP_TYPE_PARTNER:
		return "partner"
	case accountingpb.RelationshipType_RELATIONSHIP_TYPE_FRANCHISE:
		return "franchise"
	default:
		return ""
	}
}

// ✅ NEW: Convert optional relationship type
func convertOptionalRelationshipTypeToDomain(rt *accountingpb.RelationshipType) *string {
	if rt == nil {
		return nil
	}
	result := convertRelationshipTypeToDomain(*rt)
	if result == "" {
		return nil
	}
	return &result
}

func convertRelationshipTypeToProto(rt string) accountingpb.RelationshipType {
	switch rt {
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

// ✅ NEW: Convert agent status to domain
func convertAgentStatusToDomain(status accountingpb.AgentStatus) string {
	switch status {
	case accountingpb.AgentStatus_AGENT_STATUS_ACTIVE: 
		return "active"
	case accountingpb.AgentStatus_AGENT_STATUS_INACTIVE:
		return "inactive"
	case accountingpb.AgentStatus_AGENT_STATUS_DELETED:
		return "deleted"
	default:
		return "active" // default
	}
}

// ✅ NEW: Convert agent status to proto
func convertAgentStatusToProto(status domain.AgentStatus) accountingpb.AgentStatus {
	switch status {
	case domain.AgentStatusActive:
		return accountingpb.AgentStatus_AGENT_STATUS_ACTIVE
	case domain.AgentStatusInactive:
		return accountingpb. AgentStatus_AGENT_STATUS_INACTIVE
	case domain. AgentStatusDeleted:
		return accountingpb.AgentStatus_AGENT_STATUS_DELETED
	default:
		return accountingpb.AgentStatus_AGENT_STATUS_UNSPECIFIED
	}
}

// ✅ UPDATED: convertAgentToProto with new fields
func convertAgentToProto(a *domain.Agent) *accountingpb.Agent {
	if a == nil {
		return nil
	}

	// Convert metadata from map[string]interface{} to map<string,string>
	metadata := make(map[string]string)
	for k, v := range a. Metadata {
		metadata[k] = fmt.Sprint(v)
	}

	agent := &accountingpb.Agent{
		AgentExternalId:   a.AgentExternalID,
		RelationshipType: convertRelationshipTypeToProto(a.RelationshipType),
		IsActive:         a.IsActive,
		Metadata:         metadata,
		CreatedAt:        timestamppb.New(a. CreatedAt),
		UpdatedAt:        timestamppb. New(a.UpdatedAt),
		Location:         a.Location,                             // ✅ NEW
		Status:           convertAgentStatusToProto(a.Status),    // ✅ NEW
	}

	if a.UserExternalID != nil {
		agent.UserExternalId = a.UserExternalID
	}

	if a.Service != nil {
		agent.Service = a.Service
	}

	if a.CommissionRate != nil {
		rate := a.CommissionRate.String()
		agent.CommissionRate = &rate
	}

	// ✅ NEW: commission_rate_for_deposit
	if a.CommissionRateForDeposit != nil {
		rate := a.CommissionRateForDeposit.String()
		agent.CommissionRateForDeposit = &rate
	}

	if a.Name != nil {
		agent.Name = a.Name
	}

	// ✅ NEW: payment_method
	if a.PaymentMethod != nil {
		agent. PaymentMethod = a.PaymentMethod
	}

	// ✅ NEW: Include accounts if present
	if len(a.Accounts) > 0 {
		agent. Accounts = convertAccountsToProto(a. Accounts)
	}

	return agent
}

func convertAgentsToProto(agents []*domain.Agent) []*accountingpb.Agent {
	result := make([]*accountingpb.Agent, len(agents))
	for i, agent := range agents {
		result[i] = convertAgentToProto(agent)
	}
	return result
}

func convertAgentCommissionToProto(ac *domain.AgentCommission) *accountingpb.AgentCommission {
	if ac == nil {
		return nil
	}

	commission := &accountingpb.AgentCommission{
		Id:                ac.ID,
		AgentExternalId:   ac. AgentExternalID,
		UserExternalId:    ac.UserExternalID,
		AgentAccountId:    ac.AgentAccountID,
		UserAccountId:     ac.UserAccountID,
		ReceiptCode:       ac.ReceiptCode,
		TransactionAmount: ac.TransactionAmount. String(),
		CommissionRate:    ac.CommissionRate.String(),
		CommissionAmount:  ac.CommissionAmount.String(),
		Currency:          ac.Currency,
		PaidOut:           ac.PaidOut,
		CreatedAt:         timestamppb.New(ac.CreatedAt),
	}

	if ac.PayoutReceiptCode != "" {
		commission.PayoutReceiptCode = &ac.PayoutReceiptCode
	}

	if ac.PaidOutAt != nil {
		commission.PaidOutAt = timestamppb.New(*ac.PaidOutAt)
	}

	return commission
}

func convertAgentCommissionsToProto(commissions []*domain.AgentCommission) []*accountingpb.AgentCommission {
	result := make([]*accountingpb.AgentCommission, len(commissions))
	for i, commission := range commissions {
		result[i] = convertAgentCommissionToProto(commission)
	}
	return result
}

func convertOptionalBool(b *bool) *bool {
	if b == nil {
		return nil
	}
	return b
}
