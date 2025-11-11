package hgrpc

import (
	"context"
	"log"

	"ptn-rbac-service/internal/domain"
	"ptn-rbac-service/internal/repository"
	rbacpb "x/shared/genproto/partner/ptnrbacpb"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AssignUserRole assigns a role to a user
func (h *RBACGRPCHandler) AssignUserRole(ctx context.Context, req *rbacpb.AssignUserRoleRequest) (*emptypb.Empty, error) {
	log.Printf("[gRPC] AssignUserRole request: %+v", req)

	userRole := &domain.UserRole{
		UserID:     req.GetUserId(),
		RoleID:     req.GetRoleId(),
		AssignedBy: req.GetAssignedBy(),
	}

	_, perr, err := h.rbacUC.AssignUserRoles(ctx, []*domain.UserRole{userRole})
	if err != nil {
		log.Printf("❌ AssignUserRole failed: %v", err)
		return nil, err
	}
	if len(perr) > 0 {
		log.Printf("⚠️ AssignUserRole partial errors: %+v", perr)
	}

	return &emptypb.Empty{}, nil
}

// RemoveUserRole removes a role from a user
func (h *RBACGRPCHandler) RemoveUserRole(ctx context.Context, req *rbacpb.RemoveUserRoleRequest) (*emptypb.Empty, error) {
	log.Printf("[gRPC] RemoveUserRole request: %+v", req)

	if _, err := h.rbacUC.UpgradeUserRole(ctx, req.GetUserId(), req.GetRoleId(), 0); err != nil {
		log.Printf("❌ RemoveUserRole failed: %v", err)
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// ListUserRoles lists all roles assigned to a user
func (h *RBACGRPCHandler) ListUserRoles(ctx context.Context, req *rbacpb.ListUserRolesRequest) (*rbacpb.ListUserRolesResponse, error) {
	log.Printf("[gRPC] ListUserRoles request: %+v", req)

	userRoles, err := h.rbacUC.ListUserRoles(ctx, req.GetUserId())
	if err != nil {
		log.Printf("❌ ListUserRoles failed: %v", err)
		return nil, err
	}

	var pbRoles []*rbacpb.UserRole
	for _, ur := range userRoles {
		pbRoles = append(pbRoles, &rbacpb.UserRole{
			Id:         ur.ID,
			UserId:     ur.UserID,
			RoleId:     ur.RoleID,
			RoleName:   ur.RoleName,
			AssignedBy: ur.AssignedBy,
			CreatedAt:  timestamppb.New(ur.CreatedAt),
			UpdatedAt:  ptrToTimestamp(ur.UpdatedAt),
			UpdatedBy:  ptrToInt64(ur.UpdatedBy),
		})
	}

	return &rbacpb.ListUserRolesResponse{Roles: pbRoles}, nil
}

// UpgradeUserRole upgrades or replaces a user's role
func (h *RBACGRPCHandler) UpgradeUserRole(ctx context.Context, req *rbacpb.UpgradeUserRoleRequest) (*rbacpb.UserRoleResponse, error) {
	log.Printf("[gRPC] UpgradeUserRole request: %+v", req)

	ur, err := h.rbacUC.UpgradeUserRole(ctx, req.GetUserId(), req.GetNewRoleId(), req.GetAssignedBy())
	if err != nil {
		log.Printf("❌ UpgradeUserRole failed: %v", err)
		return nil, err
	}

	return &rbacpb.UserRoleResponse{
		Role: &rbacpb.UserRole{
			Id:         ur.ID,
			UserId:     ur.UserID,
			RoleId:     ur.RoleID,
			AssignedBy: ur.AssignedBy,
			CreatedAt:  timestamppb.New(ur.CreatedAt),
			UpdatedAt:  ptrToTimestamp(ur.UpdatedAt),
			UpdatedBy:  ptrToInt64(ur.UpdatedBy),
		},
	}, nil
}

// ---------------------- USER PERMISSION OVERRIDES ----------------------

// AssignUserPermissionOverride assigns a permission override to a user
func (h *RBACGRPCHandler) AssignUserPermissionOverride(ctx context.Context, req *rbacpb.AssignUserPermissionOverrideRequest) (*emptypb.Empty, error) {
	log.Printf("[gRPC] AssignUserPermissionOverride request: %+v", req)

	override := &domain.UserPermissionOverride{
		UserID:           req.GetUserId(),
		ModuleID:         req.GetModuleId(),
		SubmoduleID:      Int64Ptr(req.GetSubmoduleId()),
		PermissionTypeID: req.GetPermissionTypeId(),
		Allow:            req.GetAllow(),
		CreatedBy:        req.GetCreatedBy(),
	}

	_, perr, err := h.rbacUC.AssignUserPermissionOverrides(ctx, []*domain.UserPermissionOverride{override})
	if err != nil {
		log.Printf("❌ AssignUserPermissionOverride failed: %v", err)
		return nil, err
	}
	if len(perr) > 0 {
		log.Printf("⚠️ AssignUserPermissionOverride partial errors: %+v", perr)
	}

	return &emptypb.Empty{}, nil
}

// RevokeUserPermissionOverride revokes a permission override from a user
func (h *RBACGRPCHandler) RevokeUserPermissionOverride(ctx context.Context, req *rbacpb.RevokeUserPermissionOverrideRequest) (*emptypb.Empty, error) {
	log.Printf("[gRPC] RevokeUserPermissionOverride request: %+v", req)

	override := &domain.UserPermissionOverride{
		UserID:           req.GetUserId(),
		ModuleID:         req.GetModuleId(),
		SubmoduleID:      Int64Ptr(req.GetSubmoduleId()),
		PermissionTypeID: req.GetPermissionTypeId(),
	}

	_, perr, err := h.rbacUC.AssignUserPermissionOverrides(ctx, []*domain.UserPermissionOverride{override})
	if err != nil {
		log.Printf("❌ RevokeUserPermissionOverride failed: %v", err)
		return nil, err
	}
	if len(perr) > 0 {
		log.Printf("⚠️ RevokeUserPermissionOverride partial errors: %+v", perr)
	}

	return &emptypb.Empty{}, nil
}

// ListUserPermissionOverrides lists all overrides for a user
func (h *RBACGRPCHandler) ListUserPermissionOverrides(ctx context.Context, req *rbacpb.ListUserPermissionOverridesRequest) (*rbacpb.ListUserPermissionOverridesResponse, error) {
	log.Printf("[gRPC] ListUserPermissionOverrides request: %+v", req)

	overrides, err := h.rbacUC.ListUserPermissionOverrides(ctx, req.GetUserId())
	if err != nil {
		log.Printf("❌ ListUserPermissionOverrides failed: %v", err)
		return nil, err
	}

	var pbOverrides []*rbacpb.UserPermissionOverride
	for _, o := range overrides {
		pbOverrides = append(pbOverrides, &rbacpb.UserPermissionOverride{
			Id:               o.ID,
			UserId:           o.UserID,
			ModuleId:         o.ModuleID,
			SubmoduleId:      int64PtrToProto(o.SubmoduleID),
			PermissionTypeId: o.PermissionTypeID,
			Allow:            o.Allow,
			CreatedAt:        timestamppb.New(o.CreatedAt),
			UpdatedAt:        ptrToTimestamp(o.UpdatedAt),
		})
	}

	return &rbacpb.ListUserPermissionOverridesResponse{Overrides: pbOverrides}, nil
}

// ---------------------- EFFECTIVE USER PERMISSIONS ----------------------

// GetEffectiveUserPermissions returns all effective permissions for a user
func (h *RBACGRPCHandler) GetEffectiveUserPermissions(
	ctx context.Context,
	req *rbacpb.GetEffectiveUserPermissionsRequest,
) (*rbacpb.GetEffectiveUserPermissionsResponse, error) {
	log.Printf("[gRPC] GetEffectiveUserPermissions request: %+v", req)

	// Fetch
	perms, err := h.rbacUC.GetEffectivePermissions(ctx, req.GetUserId(), nil, nil)
	if err != nil {
		log.Printf("❌ GetEffectiveUserPermissions failed: %v", err)
		return nil, err
	}

	// Group
	grouped := repository.GroupEffectivePermissions(perms)

	// Map grouped → proto
	var pbModules []*rbacpb.ModuleWithPermissions
	for _, m := range grouped {
		pbMod := &rbacpb.ModuleWithPermissions{
			Id:          m.ID,
			Code:        m.Code,
			Name:        m.Name,
			IsActive:    m.IsActive,
			Permissions: make([]*rbacpb.PermissionInfo, 0, len(m.Permissions)),
			Submodules:  make([]*rbacpb.SubmoduleWithPermissions, 0, len(m.Submodules)),
		}

		for _, p := range m.Permissions {
			pbMod.Permissions = append(pbMod.Permissions, &rbacpb.PermissionInfo{
				Id:      p.ID,
				Code:    p.Code,
				Name:    p.Name,
				Allowed: p.Allowed,
				RoleId:  int64OrDefault(p.RoleID),
				UserId:  p.UserID,
			})
		}

		for _, sm := range m.Submodules {
			pbSm := &rbacpb.SubmoduleWithPermissions{
				Id:          int64OrDefault(sm.ID),
				Code:        stringOrDefault(sm.Code),
				Name:        stringOrDefault(sm.Name),
				IsActive:    boolOrDefault(sm.IsActive),
				Permissions: make([]*rbacpb.PermissionInfo, 0, len(sm.Permissions)),
			}

			for _, p := range sm.Permissions {
				pbSm.Permissions = append(pbSm.Permissions, &rbacpb.PermissionInfo{
					Id:      p.ID,
					Code:    p.Code,
					Name:    p.Name,
					Allowed: p.Allowed,
					RoleId:  int64OrDefault(p.RoleID),
					UserId:  p.UserID,
				})
			}

			pbMod.Submodules = append(pbMod.Submodules, pbSm)
		}

		pbModules = append(pbModules, pbMod)
	}

	return &rbacpb.GetEffectiveUserPermissionsResponse{
		UserId:  req.GetUserId(),
		Modules: pbModules,
	}, nil
}

// CheckUserPermission checks if a user has a specific permission
func (h *RBACGRPCHandler) CheckUserPermission(ctx context.Context, req *rbacpb.CheckUserPermissionRequest) (*rbacpb.CheckUserPermissionResponse, error) {
	log.Printf("[gRPC] CheckUserPermission request: %+v", req)

	allowed, err := h.rbacUC.CheckUserPermission(ctx, req.GetUserId(), req.GetModuleId(), req.GetSubmoduleId(), req.GetPermissionTypeId())
	if err != nil {
		log.Printf("❌ CheckUserPermission failed: %v", err)
		return nil, err
	}

	return &rbacpb.CheckUserPermissionResponse{
		Allowed: allowed,
	}, nil
}
