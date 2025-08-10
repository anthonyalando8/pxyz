package handler
import (
	"net/http"
	"x/shared/auth/middleware"
	"x/shared/response"
)

func (h *AuthHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	requestedUserID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || requestedUserID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	user, err := h.uc.GetProfile(r.Context(), requestedUserID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve user profile")
		return
	}
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"user_id":    user.ID,
		"email":      *user.Email,
		"first_name": *user.FirstName,
		"last_name":  *user.LastName,
		"phone":      user.Phone,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
		"is_verified": user.IsVerified,
		"account_status": user.AccountStatus,
	})
}