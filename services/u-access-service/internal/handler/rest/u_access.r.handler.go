package hrest

import (
	"encoding/json"
	"net/http"
	"x/shared/response"
	"u-rbac-service/internal/usecase"
)

type ModuleHandler struct{
	uc *usecase.ModuleUsecase;
}

// NewModuleHandler initializes a new ModuleHandler
func NewModuleHandler(uc *usecase.ModuleUsecase,) *ModuleHandler {
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
