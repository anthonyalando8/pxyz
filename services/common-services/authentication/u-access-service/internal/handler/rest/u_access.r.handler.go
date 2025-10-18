package hrest

import (
	"encoding/json"
	"log"
	"net/http"
	"u-rbac-service/internal/repository"
	"u-rbac-service/internal/usecase"
	"x/shared/auth/middleware"
	"x/shared/response"
)

type ModuleHandler struct{
	uc *usecase.RBACUsecase;
}

// NewModuleHandler initializes a new ModuleHandler
func NewModuleHandler(uc *usecase.RBACUsecase,) *ModuleHandler {
	return &ModuleHandler{
		uc: uc,
	}
}

// CreateModule handles HTTP POST /modules
func (h *ModuleHandler) HandleCreateModule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code      string                 `json:"code"`
		Name      string                 `json:"name"`
		Meta      map[string]interface{} `json:"meta"`
		CreatedBy int64                  `json:"created_by"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Placeholder response (no usecase interaction yet)
	resp := map[string]interface{}{
		"id":         12345, // mock ID
		"code":       req.Code,
		"name":       req.Name,
		"meta":       req.Meta,
		"created_by": req.CreatedBy,
		"created_at": 1700000000, // mock timestamp
	}

	response.JSON(w, http.StatusCreated, resp)
}

// HandleGetUserPermissions handles GET /users/me/permissions
func (h *ModuleHandler) HandleGetUserPermissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	//  Extract user ID from context (middleware should set this)
	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "missing or invalid user ID")
		return
	}
	moduleCode := r.URL.Query().Get("mod")
	submoduleCode := r.URL.Query().Get("subm")

	var modPtr, submPtr *string
	if moduleCode != "" {
		modPtr = &moduleCode
	}
	if submoduleCode != "" {
		submPtr = &submoduleCode
	}

	//  Fetch effective permissions
	perms, err := h.uc.GetEffectivePermissions(ctx, userID, modPtr, submPtr)
	if err != nil {
		log.Printf("❌ HandleGetUserPermissions failed: %v", err)
		response.Error(w, http.StatusInternalServerError, "failed to fetch permissions")
		return
	}

	//  Group by module → submodule → permissions
	grouped := repository.GroupEffectivePermissions(perms)

	//  Response
	resp := map[string]interface{}{
		"user_id":     userID,
		"modules": grouped,
	}

	response.JSON(w, http.StatusOK, resp)
}
