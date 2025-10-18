package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	rbacpb "x/shared/genproto/urbacpb"
	"x/shared/response"
)

// ---------------- USER PERMISSION OVERRIDES ----------------

type assignPermissionRequest struct {
	UserID           string `json:"user_id"`
	ModuleID         int64  `json:"module_id"`
	SubmoduleID      int64  `json:"submodule_id"`
	PermissionTypeID int64  `json:"permission_type_id"`
	Allow            bool   `json:"allow"`
	CreatedBy        int64  `json:"created_by"`
}

type revokePermissionRequest struct {
	UserID           string `json:"user_id"`
	ModuleID         int64  `json:"module_id"`
	SubmoduleID      int64  `json:"submodule_id"`
	PermissionTypeID int64  `json:"permission_type_id"`
}

func (h *AdminHandler) HandleRevokeUserPermission(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req revokePermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	_, err := h.auth.RBACClient.RevokeUserPermissionOverride(ctx, &rbacpb.RevokeUserPermissionOverrideRequest{
		UserId:           req.UserID,
		ModuleId:         req.ModuleID,
		SubmoduleId:      req.SubmoduleID,
		PermissionTypeId: req.PermissionTypeID,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to revoke permission")
		return
	}

	// Attempt to clear Redis cache (ignore errors)
	cacheKeys := []string{
		fmt.Sprintf("rbac:effective_permissions:%s", req.UserID),
		"urbac:user:effective_perms:" + req.UserID,
	}
	for _, key := range cacheKeys {
		_ = h.redisClient.Del(ctx, key).Err()
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"status": "permission revoked",
	})
}

func (h *AdminHandler) HandleAssignUserPermission(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req assignPermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	_, err := h.auth.RBACClient.AssignUserPermissionOverride(ctx, &rbacpb.AssignUserPermissionOverrideRequest{
		UserId:           req.UserID,
		ModuleId:         req.ModuleID,
		SubmoduleId:      req.SubmoduleID,
		PermissionTypeId: req.PermissionTypeID,
		Allow:            req.Allow,
		CreatedBy:        req.CreatedBy,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to assign permission")
		return
	}

	// Attempt to clear Redis cache (ignore errors)
	cacheKeys := []string{
		fmt.Sprintf("rbac:effective_permissions:%s", req.UserID),
		"urbac:user:effective_perms:" + req.UserID,
	}
	for _, key := range cacheKeys {
		_ = h.redisClient.Del(ctx, key).Err()
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"status": "permission assigned",
	})
}

func (h *AdminHandler) HandleListUserPermissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		response.Error(w, http.StatusBadRequest, "missing user_id")
		return
	}

	res, err := h.auth.RBACClient.ListUserPermissionOverrides(ctx, &rbacpb.ListUserPermissionOverridesRequest{
		UserId: userID,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list permissions")
		return
	}

	response.JSON(w, http.StatusOK, res.Overrides)
}

// ---------------- PLACEHOLDER HANDLERS ----------------

// Modules
func (h *AdminHandler) HandleCreateModule(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleUpdateModule(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleDeactivateModule(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleDeleteModule(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleListModules(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}

// Submodules
func (h *AdminHandler) HandleCreateSubmodule(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleUpdateSubmodule(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleDeactivateSubmodule(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleDeleteSubmodule(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleListSubmodules(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}

// Permission Types
func (h *AdminHandler) HandleCreatePermissionType(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleUpdatePermissionType(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleDeactivatePermissionType(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleListPermissionTypes(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}

// Roles
func (h *AdminHandler) HandleCreateRole(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleUpdateRole(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleDeactivateRole(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleDeleteRole(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleListRoles(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}

// Role Permissions
func (h *AdminHandler) HandleAssignRolePermission(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleRevokeRolePermission(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleListRolePermissions(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}

// User Roles
func (h *AdminHandler) HandleAssignUserRole(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleRemoveUserRole(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleListUserRoles(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *AdminHandler) HandleUpgradeUserRole(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}

// Permission Queries
func (h *AdminHandler) HandleGetEffectiveUserPermissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		response.Error(w, http.StatusBadRequest, "missing user_id")
		return
	}

	cacheKey := "urbac:user:effective_perms:" + userID

	// 1️⃣ Try to get from Redis cache
	cached, err := h.redisClient.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		// Cached JSON exists, return it
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(cached))
		return
	}

	// 2️⃣ Fetch from RBAC service
	rbacResp, err := h.auth.RBACClient.GetEffectiveUserPermissions(ctx, &rbacpb.GetEffectiveUserPermissionsRequest{
		UserId: userID,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch effective permissions")
		return
	}

	// 3️⃣ Marshal response
	data, err := json.Marshal(rbacResp)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to encode response")
		return
	}

	// 4️⃣ Cache the result in Redis (best-effort, ignore errors)
	_ = h.redisClient.Set(ctx, cacheKey, data, 10*time.Minute).Err()

	// 5️⃣ Return response using response.JSON
	response.JSON(w, http.StatusOK, rbacResp)
}

func (h *AdminHandler) HandleCheckUserPermission(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}

// Audit Logs
func (h *AdminHandler) HandleListPermissionAuditEvents(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented yet")
}
