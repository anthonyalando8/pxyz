package hgrpc

import (
	"context"
	"log"

	"ptn-rbac-service/internal/domain"
	"ptn-rbac-service/internal/usecase"
	rbacpb "x/shared/genproto/partner/ptnrbacpb"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RBACGRPCHandler struct {
	rbacpb.UnimplementedRBACServiceServer
	rbacUC *usecase.RBACUsecase
}

func NewRBACGRPCHandler(uc *usecase.RBACUsecase) *RBACGRPCHandler {
	return &RBACGRPCHandler{rbacUC: uc}
}

// -------------------------- MODULES --------------------------

func (h *RBACGRPCHandler) CreateModule(ctx context.Context, req *rbacpb.CreateModuleRequest) (*rbacpb.ModuleResponse, error) {
	log.Printf("[gRPC] CreateModule request: %+v", req)

	var metaBytes []byte
	if req.GetMeta() != nil {
		metaBytes, _ = protojson.Marshal(req.GetMeta())
	}

	mod := &domain.Module{
		Code:      req.GetCode(),
		Name:      req.GetName(),
		IsActive:  req.GetIsActive(),
		CreatedBy: req.GetCreatedBy(),
		Meta:      metaBytes,
	}

	// Handle optional parent_id
	if req.ParentId != 0 {
		mod.ParentID = &req.ParentId
	}

	created, perr, err := h.rbacUC.CreateModules(ctx, []*domain.Module{mod})
	if err != nil {
		log.Printf("❌ CreateModule failed: %v", err)
		return nil, err
	}
	if len(perr) > 0 {
		log.Printf("⚠️ CreateModule partial errors: %+v", perr)
	}

	m := created[0]
	return &rbacpb.ModuleResponse{
		Module: &rbacpb.Module{
			Id:        m.ID,
			Code:      m.Code,
			Name:      m.Name,
			ParentId:  ptrToInt64(m.ParentID),
			Meta:      req.Meta, // return original struct
			IsActive:  m.IsActive,
			CreatedAt: timestamppb.New(m.CreatedAt),
			UpdatedAt: ptrToTimestamp(m.UpdatedAt),
			CreatedBy: m.CreatedBy,
			UpdatedBy: ptrToInt64(m.UpdatedBy),
		},
	}, nil
}

func (h *RBACGRPCHandler) UpdateModule(ctx context.Context, req *rbacpb.UpdateModuleRequest) (*rbacpb.ModuleResponse, error) {
	mod := &domain.Module{
		ID:   req.GetId(),
		Code: req.GetCode(),
		Name: req.GetName(),
	}

	// optional parent
	if req.ParentId != 0 {
		mod.ParentID = &req.ParentId
	}

	if err := h.rbacUC.UpdateModule(ctx, mod); err != nil {
		log.Printf("❌ UpdateModule failed: %v", err)
		return nil, err
	}

	// Fetch updated module
	updated, err := h.rbacUC.GetModuleByCode(ctx, mod.Code)
	if err != nil {
		return nil, err
	}

	return &rbacpb.ModuleResponse{
		Module: &rbacpb.Module{
			Id:        updated.ID,
			Code:      updated.Code,
			Name:      updated.Name,
			ParentId:  ptrToInt64(updated.ParentID),
			IsActive:  updated.IsActive,
			CreatedAt: timestamppb.New(updated.CreatedAt),
			UpdatedAt: ptrToTimestamp(updated.UpdatedAt),
			CreatedBy: updated.CreatedBy,
			UpdatedBy: ptrToInt64(updated.UpdatedBy),
		},
	}, nil
}

func (h *RBACGRPCHandler) ListModules(ctx context.Context, req *rbacpb.ListModulesRequest) (*rbacpb.ListModulesResponse, error) {
	mods, err := h.rbacUC.ListModules(ctx)
	if err != nil {
		return nil, err
	}

	var pbMods []*rbacpb.Module
	for _, m := range mods {
		pbMods = append(pbMods, &rbacpb.Module{
			Id:        m.ID,
			Code:      m.Code,
			Name:      m.Name,
			ParentId:  ptrToInt64(m.ParentID),
			IsActive:  m.IsActive,
			CreatedAt: timestamppb.New(m.CreatedAt),
			UpdatedAt: ptrToTimestamp(m.UpdatedAt),
			CreatedBy: m.CreatedBy,
			UpdatedBy: ptrToInt64(m.UpdatedBy),
		})
	}

	return &rbacpb.ListModulesResponse{Modules: pbMods}, nil
}

// -------------------------- PLACEHOLDER: SUBMODULES --------------------------
func (h *RBACGRPCHandler) CreateSubmodule(ctx context.Context, req *rbacpb.CreateSubmoduleRequest) (*rbacpb.SubmoduleResponse, error) {
	// TODO: implement similar to CreateModule
	return nil, nil
}

// Add all remaining handlers following the same pattern:
// UpdateSubmodule, ListSubmodules, CreateRole, UpdateRole, ListRoles, CreatePermissionType,
// AssignRolePermission, AssignUserRole, UpgradeUserRole, GetUserPermissions, etc.
