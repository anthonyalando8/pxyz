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
		"email":             safeString(user.Email),
		"first_name":        safeString(user.FirstName),
		"last_name":         safeString(user.LastName),
		"phone":             safeString(user.Phone),
		"is_email_verified": user.IsEmailVerified,
		"is_phone_verified": user.IsPhoneVerified,
		"account_type":      user.AccountType,
		"created_at":        user.CreatedAt,
		"updated_at":        user.UpdatedAt,
		"account_status":    user.AccountStatus,
	})
}