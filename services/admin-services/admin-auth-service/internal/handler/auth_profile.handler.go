package handler

import (
	"context"
	"encoding/json"
	"fmt"
	stdimage "image"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"x/shared/auth/middleware"
	"x/shared/response"
	"x/shared/utils/image"

	_ "image/gif" // register gif
	_ "image/jpeg"
	_ "image/png"

	"google.golang.org/protobuf/types/known/structpb"
)

// GetFullUserProfile fetches and merges user + account-service profile into a map
func (h *AuthHandler) GetFullUserProfile(ctx context.Context, userID string) (map[string]interface{}, error) {
	// Get user (from user table)
	user, err := h.uc.GetProfile(ctx, userID)
	if err != nil {
		log.Printf("[ERROR] Failed to retrieve user record user_id=%s error=%v", userID, err)
		return nil, err
	}
	log.Printf("[DEBUG] Retrieved user record user_id=%s email=%s", userID, safeString(user.Email))

	// Merge response
	resp := map[string]interface{}{
		"user_id":        userID,
		"email":          safeString(user.Email),
		"phone":          safeString(user.Phone),
		"account_type":   user.AccountType,
		"account_status": user.AccountStatus,

		// Extended profile
		"first_name":    user.FirstName,
		"last_name":     user.LastName,
	}

	// Add is_email_verified only if email exists
	if user.Email != nil && *user.Email != "" {
		resp["is_email_verified"] = user.IsEmailVerified
	}

	// Add is_phone_verified only if phone exists
	if user.Phone != nil && *user.Phone != "" {
		resp["is_phone_verified"] = user.IsPhoneVerified
	}

	return resp, nil
}

// ---------- HTTP Handler ----------

func (h *AuthHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	requestedUserID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || requestedUserID == "" {
		log.Printf("[WARN] Unauthorized access attempt from %s", r.RemoteAddr)
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log.Printf("[INFO] Fetching profile for user_id=%s", requestedUserID)

	resp, err := h.GetFullUserProfile(r.Context(), requestedUserID)
	if err != nil {
		log.Printf("[ERROR] Failed to build profile for user_id=%s error=%v", requestedUserID, err)
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve user profile")
		return
	}

	log.Printf("[INFO] Successfully fetched profile user_id=%s", requestedUserID)
	response.JSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) HandleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req UpdateProfileRequest
	if err := decodeRequestBody(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}


	// Call update services

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Profile updated successfully",
	})
}


// UploadProfilePicture handles profile picture uploads
func (h *AuthHandler) UploadProfilePicture(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		log.Println("[ERROR] Unauthorized upload attempt, missing userID in context")
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	log.Printf("[INFO] Starting profile picture upload for userID=%s", userID)

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("[ERROR] Failed to read form file for userID=%s: %v", userID, err)
		response.Error(w, http.StatusBadRequest, "failed to read file")
		return
	}
	defer file.Close()

	log.Printf("[DEBUG] File received: %s (%d bytes)", header.Filename, header.Size)

	// Validate file type (ensure it's an image)
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		log.Printf("[ERROR] Invalid file type: %s for userID=%s", ext, userID)
		response.Error(w, http.StatusBadRequest, "only JPG and PNG images are allowed")
		return
	}

	img, format, err := stdimage.Decode(file)
	if err != nil {
		log.Printf("[ERROR] Failed to decode image for userID=%s: %v", userID, err)
		response.Error(w, http.StatusBadRequest, "invalid image file")
		return
	}
	log.Printf("[DEBUG] Image format detected: %s", format)
	log.Printf("[DEBUG] Image format detected: %s", format)

	// Ensure upload directory exists
	uploadDir := "/app/uploads/profile_pictures"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("[ERROR] Failed to create upload dir %s: %v", uploadDir, err)
		response.Error(w, http.StatusInternalServerError, "failed to prepare upload dir")
		return
	}

	// Save path (overwrite old one for same user)
	filename := fmt.Sprintf("%s.jpg", userID) // save all as JPEG after compression
	savePath := filepath.Join(uploadDir, filename)

	// Compress and save
	if err := image.CompressAndSaveImage(img, savePath, 400, 400, 80); err != nil {
		log.Printf("[ERROR] Failed to save compressed image for userID=%s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "failed to save image")
		return
	}
	log.Printf("[INFO] Successfully saved compressed profile picture for userID=%s", userID)

	// Construct image URL
	imageURL := fmt.Sprintf("http://localhost/auth/uploads/profile_pictures/%s", filename)

	// Call Account service to update DB
	
	log.Printf("[INFO] Profile picture updated in account service for userID=%s", userID)

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success":           true,
		"message":           "Profile picture updated",
		"profile_image_url": imageURL,
	})
}



func (h *AuthHandler) HandleGetPreferences(w http.ResponseWriter, r *http.Request) {
	// --- Extract user ID from context ---
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// --- Call account service to fetch preferences ---
	

	// --- Return preferences in response ---
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"preferences": "",
	})
}

func (h *AuthHandler) HandleUpdatePreferences(w http.ResponseWriter, r *http.Request) {
	// --- Extract user ID from context ---
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// --- Parse request body ---
	var req struct {
		DarkMode            *bool `json:"dark_mode,omitempty"`
		DarkModeEmails      *bool `json:"dark_mode_emails,omitempty"`
		LocationTracking    *bool `json:"location_tracking,omitempty"`
		AutoLogin           *bool `json:"auto_login,omitempty"`
		MarketingEmails     *bool `json:"marketing_emails,omitempty"`
		PushNotifications   *bool `json:"push_notifications,omitempty"`
		SMSNotifications    *bool `json:"sms_notifications,omitempty"`
		ChartMessageSound   *bool `json:"chart_message_sound,omitempty"`
		NotificationSound   *bool `json:"notification_sound,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// --- Build map of preferences to update ---
	prefs := make(map[string]*structpb.Value)

	if req.DarkMode != nil {
		prefs["dark_mode"] = structpb.NewBoolValue(*req.DarkMode)
	}
	if req.DarkModeEmails != nil {
		prefs["dark_mode_emails"] = structpb.NewBoolValue(*req.DarkModeEmails)
	}
	if req.LocationTracking != nil {
		prefs["location_tracking"] = structpb.NewBoolValue(*req.LocationTracking)
	}
	if req.AutoLogin != nil {
		prefs["auto_login"] = structpb.NewBoolValue(*req.AutoLogin)
	}
	if req.MarketingEmails != nil {
		prefs["marketing_emails"] = structpb.NewBoolValue(*req.MarketingEmails)
	}
	if req.PushNotifications != nil {
		prefs["push_notifications"] = structpb.NewBoolValue(*req.PushNotifications)
	}
	if req.SMSNotifications != nil {
		prefs["sms_notifications"] = structpb.NewBoolValue(*req.SMSNotifications)
	}
	if req.ChartMessageSound != nil {
		prefs["chart_message_sound"] = structpb.NewBoolValue(*req.ChartMessageSound)
	}
	if req.NotificationSound != nil {
		prefs["notification_sound"] = structpb.NewBoolValue(*req.NotificationSound)
	}

	if len(prefs) == 0 {
		response.Error(w, http.StatusBadRequest, "No valid preferences provided")
		return
	}

	// --- Call account service to update preferences ---
	

	// --- Success response ---
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "Preferences updated successfully",
	})
}

