package hgrpc

import (
	"context"

	"ptn-rbac-service/internal/domain"
	rbacpb "x/shared/genproto/partner/ptnrbacpb"

	"google.golang.org/protobuf/types/known/emptypb"
)

func (h *RBACGRPCHandler) CreateRole(ctx context.Context, req *rbacpb.CreateRoleRequest) (*rbacpb.RoleResponse, error) {
	role := &domain.Role{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		IsActive:    true,
	}

	roles, _, err := h.rbacUC.CreateRoles(ctx, []*domain.Role{role})
	if err != nil {
		return nil, err
	}

	return &rbacpb.RoleResponse{
		Role: &rbacpb.Role{
			Id:          roles[0].ID,
			Name:        roles[0].Name,
			Description: roles[0].Description,
			IsActive:    roles[0].IsActive,
			CreatedAt:   ptrToTimestamp(&roles[0].CreatedAt),
			UpdatedAt:   ptrToTimestamp(roles[0].UpdatedAt),
		},
	}, nil
}

func (h *RBACGRPCHandler) UpdateRole(ctx context.Context, req *rbacpb.UpdateRoleRequest) (*rbacpb.RoleResponse, error) {
	role := &domain.Role{
		ID:          req.GetId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
	}

	if err := h.rbacUC.UpdateRole(ctx, role); err != nil {
		return nil, err
	}

	return &rbacpb.RoleResponse{
		Role: &rbacpb.Role{
			Id:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			IsActive:    role.IsActive,
			UpdatedAt:   ptrToTimestamp(role.UpdatedAt),
		},
	}, nil
}

func (h *RBACGRPCHandler) DeactivateRole(ctx context.Context, req *rbacpb.DeactivateRoleRequest) (*emptypb.Empty, error) {
	// just call update with IsActive=false
	role := &domain.Role{
		ID:       req.GetId(),
		IsActive: false,
	}

	if err := h.rbacUC.UpdateRole(ctx, role); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (h *RBACGRPCHandler) ListRoles(ctx context.Context, _ *emptypb.Empty) (*rbacpb.ListRolesResponse, error) {
	roles, err := h.rbacUC.ListRoles(ctx)
	if err != nil {
		return nil, err
	}

	resp := &rbacpb.ListRolesResponse{}
	for _, r := range roles {
		resp.Roles = append(resp.Roles, &rbacpb.Role{
			Id:          r.ID,
			Name:        r.Name,
			Description: r.Description,
			IsActive:    r.IsActive,
			CreatedAt:   ptrToTimestamp(&r.CreatedAt),
			UpdatedAt:   ptrToTimestamp(r.UpdatedAt),
		})
	}

	return resp, nil
}
