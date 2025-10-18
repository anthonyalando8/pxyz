package handler

import (
	"auth-service/internal/domain"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	//"x/shared/auth/middleware"
	"x/shared/genproto/accountpb"
	"x/shared/response"
)

type Enable2FARequest struct {
	Method string `json:"method"`
	Secret string `json:"secret"`
	Code   string `json:"totp_code"`
}

type Disable2FARequest struct {
	Method     string `json:"method"`
	Code       string `json:"totp_code"`
	BackupCode string `json:"backup_code"`
}

type Verify2FARequest struct {
	Method     string `json:"method"`
	Code       string `json:"totp_code"`
	BackupCode string `json:"backup_code"`
}


// --- Helper: Determine communication target (email/phone) ---
func getComTarget(user *domain.UserProfile, emailOnly bool) (string, error) {
	if user.Email != nil && *user.Email != "" {
		return *user.Email, nil
	}
	if !emailOnly {
		if user.Phone != nil && *user.Phone != "" {
			return *user.Phone, nil
		}
	}

	return "", fmt.Errorf("no valid identifier available")
}

// --- Helper: Decode request body into target struct ---
func decodeRequestBody(r *http.Request, target interface{}) error {
	return json.NewDecoder(r.Body).Decode(target)
}

func (h *AuthHandler) HandleInitiate2FA(w http.ResponseWriter, r *http.Request) {
	userID, user, ok := h.getUserFromContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	totpId, err := getComTarget(user, false)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.accountClient.Client.InitiateTOTPSetup(r.Context(), &accountpb.InitiateTOTPSetupRequest{
		UserId: userID,
		Email:  totpId,
	})
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"totp_url":    resp.OtpUrl,
		"totp_secret": resp.Secret,
		"next":        "scan_code",
	})
}

func (h *AuthHandler) HandleEnable2FA(w http.ResponseWriter, r *http.Request) {
	userID, user, ok := h.getUserFromContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req Enable2FARequest
	if err := decodeRequestBody(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Code == "" || req.Secret == "" {
		response.Error(w, http.StatusBadRequest, "Code and secret required")
		return
	}

	comTarget, _ := getComTarget(user, true) // optional, empty string allowed
	resp, err := h.accountClient.Client.EnableTwoFA(r.Context(), &accountpb.EnableTwoFARequest{
		UserId:     userID,
		Secret:     req.Secret,
		Code:       req.Code,
		Method:     "totp",
		ComChannel: "email",
		ComTarget:  comTarget,
	})

	if err != nil {
		log.Printf("EnableTwoFA RPC failed for user %s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "2FA enablement failed. Try again later")
		return
	}

	if !resp.Success {
		log.Printf("EnableTwoFA response unsuccessful for user %s: %+v", userID, resp)
		response.Error(w, http.StatusInternalServerError, "2FA enablement failed. Try again later")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message":     "2FA added successfully.",
		"backup_code": resp.BackupCodes,
	})
}

func (h *AuthHandler) HandleDisable2FA(w http.ResponseWriter, r *http.Request) {
	userID, user, ok := h.getUserFromContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req Disable2FARequest
	if err := decodeRequestBody(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Code == "" && req.BackupCode == "" {
		response.Error(w, http.StatusBadRequest, "TOTP code or backup codes required")
		return
	}

	comTarget, _ := getComTarget(user, true) // optional, empty string allowed
	resp, err := h.accountClient.Client.DisableTwoFA(r.Context(), &accountpb.DisableTwoFARequest{
		UserId:     userID,
		Code:       req.Code,
		Method:     "totp",
		BackupCode: req.BackupCode,
		ComChannel: "email",
		ComTarget:  comTarget,
	})

	if err != nil {
		log.Printf("EnableTwoFA RPC failed for user %s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "2FA disable failed. Try again later")
		return
	}

	if !resp.Success {
		log.Printf("EnableTwoFA response unsuccessful for user %s: %+v", userID, resp)
		response.Error(w, http.StatusInternalServerError, "2FA disable failed. Try again later")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "2FA disabled successfully.",
	})
}

func (h *AuthHandler) HandleVerify2FA(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := h.getUserFromContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req Disable2FARequest
	if err := decodeRequestBody(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Code == "" && req.BackupCode == "" {
		response.Error(w, http.StatusBadRequest, "TOTP code or backup codes required")
		return
	}

	resp, err := h.accountClient.Client.VerifyTwoFA(r.Context(), &accountpb.VerifyTwoFARequest{
		UserId:     userID,
		Code:       req.Code,
		BackupCode: req.BackupCode,
		Method:     "totp",
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !resp.Success {
		response.Error(w, http.StatusUnauthorized, "Verification failed. Invalid code.")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "Success",
		"valid":   resp.Success,
	})
}

func (h *AuthHandler) Handle2FAStatus(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := h.getUserFromContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	resp, err := h.accountClient.Client.GetTwoFAStatus(r.Context(), &accountpb.GetTwoFAStatusRequest{
		UserId: userID,
	})
	if err != nil{
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "Success",
		"enabled":   resp.IsEnabled,
	})
}
