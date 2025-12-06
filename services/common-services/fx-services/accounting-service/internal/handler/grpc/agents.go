package hgrpc

import (
	"context"
	//"errors"
	"fmt"
	// "reflect"
	// "strconv"
	// "time"

	//log "github.com/sirupsen/logrus"

	"accounting-service/internal/domain"
	"accounting-service/internal/usecase"
	accountingpb "x/shared/genproto/shared/accounting/v1"
	//xerrors "x/shared/utils/errors"

	//"github.com/redis/go-redis/v9"
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
		UserExternalID:   req.UserExternalId,
		Service:          req.Service,
		CommissionRate:   req.CommissionRate,
		RelationshipType: convertRelationshipTypeToDomain(req.RelationshipType),
		IsActive:         req.IsActive,
		Name:             req.Name,
		Metadata:         metadata,
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
	req *accountingpb.UpdateAgentRequest,
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
		UserExternalID: req.UserExternalId,
		Service:        req.Service,
		CommissionRate: req.CommissionRate,
		IsActive:       convertOptionalBool(req.IsActive),
		Name:           req.Name,
		Metadata:       metadata,
	}

	// Convert relationship type if provided
	if req.RelationshipType != nil {
		relType := convertRelationshipTypeToDomain(*req.RelationshipType)
		updateReq. RelationshipType = &relType
	}

	// Execute
	agent, err := h.agentUC.UpdateAgent(ctx, req.AgentExternalId, updateReq)
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
		return nil, status. Error(codes.InvalidArgument, "agent_external_id is required")
	}

	// Execute
	if err := h.agentUC.DeleteAgent(ctx, req. AgentExternalId); err != nil {
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

	// ✅ Pass includeAccounts flag to usecase
	agent, err := h.agentUC.GetAgentByID(ctx, req. AgentExternalId, req. IncludeAccounts)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetAgentByIDResponse{
		Agent: convertAgentToProto(agent),
	}, nil
}

func (h *AccountingHandler) GetAgentByUserID(
	ctx context.Context,
	req *accountingpb.GetAgentByUserIDRequest,
) (*accountingpb.GetAgentByUserIDResponse, error) {
	// Validate
	if req.UserExternalId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_external_id is required")
	}

	// ✅ Pass includeAccounts flag to usecase
	agent, err := h.agentUC.GetAgentByUserID(ctx, req.UserExternalId, req.IncludeAccounts)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetAgentByUserIDResponse{
		Agent: convertAgentToProto(agent),
	}, nil
}

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

	// ✅ Pass includeAccounts flag to usecase
	agents, err := h.agentUC.ListAgents(ctx, limit, offset, req. IncludeAccounts)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.ListAgentsResponse{
		Agents: convertAgentsToProto(agents),
		Total:  int32(len(agents)),
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

	return &accountingpb. ListCommissionsForAgentResponse{
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

func convertAgentToProto(a *domain.Agent) *accountingpb.Agent {
	if a == nil {
		return nil
	}

	// Convert metadata from map[string]interface{} to map<string,string>
	metadata := make(map[string]string)
	for k, v := range a.Metadata {
		metadata[k] = fmt.Sprint(v)
	}

	agent := &accountingpb.Agent{
		AgentExternalId:  a.AgentExternalID,
		RelationshipType: convertRelationshipTypeToProto(a.RelationshipType),
		IsActive:         a.IsActive,
		Metadata:         metadata,
		CreatedAt:        timestamppb.New(a.CreatedAt),
		UpdatedAt:        timestamppb.New(a.UpdatedAt),
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

	if a.Name != nil {
		agent.Name = a.Name
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
		AgentExternalId:   ac.AgentExternalID,
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