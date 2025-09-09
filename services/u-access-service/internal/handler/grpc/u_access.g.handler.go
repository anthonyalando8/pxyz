package hgrpc

import (
	"context"
	"log"

	"u-rbac-service/internal/domain"
	"u-rbac-service/internal/usecase"
	rbacpb "x/shared/genproto/urbacpb"

	"google.golang.org/protobuf/types/known/timestamppb"
	 "google.golang.org/protobuf/encoding/protojson"
)

type ModuleGRPCHandler struct {
	rbacpb.UnimplementedRBACServiceServer
	moduleUC *usecase.ModuleUsecase
}

func NewModuleGRPCHandler(uc *usecase.ModuleUsecase) *ModuleGRPCHandler {
	return &ModuleGRPCHandler{moduleUC: uc}
}

// CreateModule handles creation of a single module
func (h *ModuleGRPCHandler) CreateModule(ctx context.Context, req *rbacpb.CreateModuleRequest) (*rbacpb.ModuleResponse, error) {
	log.Printf("[gRPC] CreateModule request: %+v", req)

	var metaBytes []byte
	if req.GetMeta() != nil {
		metaBytes, _ = protojson.Marshal(req.GetMeta())
	}
	// Map proto → domain
	mod := &domain.Module{
		Code:      req.GetCode(),
		Name:      req.GetName(),
		Meta:      metaBytes,
		CreatedBy: req.GetCreatedBy(),
	}

	created, perr, err := h.moduleUC.CreateModules(ctx, []*domain.Module{mod})
	if err != nil {
		log.Printf("❌ CreateModule failed: %v", err)
		return nil, err
	}
	if len(perr) > 0 {
		log.Printf("⚠️ CreateModule partial errors: %+v", perr)
	}

	m := created[0]

	// Success → return first element
	return &rbacpb.ModuleResponse{
		Module: &rbacpb.Module{
			Id:        m.ID,
			Code:      m.Code,
			Name:      m.Name,
			IsActive:  m.IsActive,
			CreatedAt: timestamppb.New(m.CreatedAt),
			UpdatedAt: ptrToTimestamp(m.UpdatedAt),
		},
	}, nil
}

