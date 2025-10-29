package handler

import (
	"auth-service/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdimage "image"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"x/shared/auth/middleware"
	accountclient "x/shared/genproto/accountpb"
	"x/shared/response"
	xerrors "x/shared/utils/errors"
	"x/shared/utils/image"

	_ "image/gif" // register gif
	_ "image/jpeg"
	_ "image/png"

	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/structpb"
)

func (h *AuthHandler) postAccountCreationTask(userID, currentRole string) {
	go func(userID, currentRole string) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// 2a. Role upgrade
		if currentRole == "any" {
			if err := h.handleRoleUpgrade(bgCtx, userID, currentRole); err != nil {
				log.Printf("[SetPassword] ⚠️ Role upgrade failed for user %s: %v", userID, err)
			} else {
				log.Printf("[SetPassword] ✅ Role upgraded successfully for user %s", userID)
			}
		}

		// 2b. Profile cache refresh
		if _, err := h.GetFullUserProfile(bgCtx, userID); err != nil {
			log.Printf("[SetPassword] ⚠️ Failed to refresh profile cache for user %s: %v", userID, err)
		}
	}(userID, currentRole)
}

// GetFullUserProfile fetches and merges user + account-service profile into a map
func (h *AuthHandler) GetFullUserProfile(ctx context.Context, userID string) (*domain.UserProfile, error) {
	cacheKey := fmt.Sprintf("user_profile:%s", userID)

	// 1️⃣ Try cache first
	if cached, err := h.redisClient.Get(ctx, cacheKey).Result(); err == nil && cached != "" {
		var user domain.UserProfile
		if err := json.Unmarshal([]byte(cached), &user); err == nil {
			log.Printf("[CACHE HIT] user_id=%s", userID)
			return &user, nil
		}
		log.Printf("[CACHE ERROR] Failed to unmarshal cache user_id=%s error=%v", userID, err)
	} else {
		log.Printf("[CACHE MISS] user_id=%s", userID)
	}

	// 2️⃣ Run local DB and gRPC fetch concurrently
	var (
		user    *domain.UserProfile
		profile *accountclient.UserProfile
	)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		u, err := h.uc.GetProfile(ctx, userID)
		if err != nil {
			log.Printf("[ERROR] Failed to retrieve user from DB user_id=%s error=%v", userID, err)
			return err
		}
		user = u
		return nil
	})

	g.Go(func() error {
		resp, err := h.accountClient.Client.GetUserProfile(ctx, &accountclient.GetUserProfileRequest{
			UserId: userID,
		})
		if err != nil {
			log.Printf("[ERROR] Failed to fetch profile via gRPC user_id=%s error=%v", userID, err)
			return err
		}
		if resp == nil || resp.Profile == nil {
			return fmt.Errorf("profile not found")
		}
		profile = resp.Profile
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// 3️⃣ Merge results
	if profile != nil && user != nil {
		user.FirstName = profile.FirstName
		user.LastName = profile.LastName
		user.Username = profile.Username
		user.Bio = profile.Bio
		user.Gender = profile.Gender
		user.DateOfBirth = profile.DateOfBirth
		user.ProfileImageUrl = profile.ProfileImageUrl
		user.Nationality = profile.Nationality
		user.Address = profile.AddressJson
	}

	// 4️⃣ Cache combined result
	if user != nil {
		data, _ := json.Marshal(user)
		if err := h.redisClient.Set(ctx, cacheKey, data, 15*time.Minute).Err(); err != nil {
			log.Printf("[CACHE SET ERROR] Failed to cache user_id=%s error=%v", userID, err)
		} else {
			log.Printf("[CACHE SET] Cached user profile user_id=%s", userID)
		}
	}

	return user, nil
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


// BuildUpdateProfileRequest constructs a gRPC UpdateProfileRequest
// from a user ID and partial UpdateProfileRequest payload.
// Only sets fields that are non-nil.
func BuildUpdateProfileRequest(userID string, req *UpdateProfileRequest) (*accountclient.UpdateProfileRequest, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	updateReq := &accountclient.UpdateProfileRequest{
		UserId: userID,
	}

	if req.FirstName != nil {
		updateReq.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		updateReq.LastName = *req.LastName
	}
	if req.Surname != nil {
		updateReq.Surname = *req.Surname
	}
	if req.SysUsername != nil {
		updateReq.SysUsername = *req.SysUsername
	}
	if req.Bio != nil {
		updateReq.Bio = *req.Bio
	}
	if req.DateOfBirth != nil {
		updateReq.DateOfBirth = *req.DateOfBirth
	}
	if req.Address != nil {
		addrBytes, err := json.Marshal(req.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid address format: %w", err)
		}
		updateReq.AddressJson = string(addrBytes)
	}

	return updateReq, nil
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

	updateReq, err := BuildUpdateProfileRequest(userID, &req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	_, err = h.accountClient.Client.UpdateProfile(r.Context(), updateReq)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update profile")
		return
	}

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
	imageURL := fmt.Sprintf("/auth/uploads/profile_pictures/%s", filename)

	// Call Account service to update DB
	_, err = h.accountClient.Client.UpdateProfilePicture(
		context.Background(),
		&accountclient.UpdateProfilePictureRequest{
			UserId:   userID,
			ImageUrl: imageURL,
		},
	)
	if err != nil {
		log.Printf("[ERROR] Failed to update account profile for userID=%s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "failed to update profile")
		return
	}
	log.Printf("[INFO] Profile picture updated in account service for userID=%s", userID)

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success":           true,
		"message":           "Profile picture updated",
		"profile_image_url": imageURL,
	})
}

func (h *AuthHandler) GetProfilePicture(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		log.Println("[ERROR] Missing userID in GetProfilePicture request")
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log.Printf("[INFO] Retrieving profile picture for userID=%s", userID)

	// Fetch profile from account service
	profileResp, err := h.accountClient.Client.GetUserProfile(r.Context(), &accountclient.GetUserProfileRequest{
		UserId: userID,
	})
	if err != nil {
		log.Printf("[ERROR] Failed to fetch profile from account service for userID=%s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "failed to fetch profile")
		return
	}
	if profileResp == nil || profileResp.Profile == nil {
		log.Printf("[WARN] No profile found for userID=%s", userID)
		response.Error(w, http.StatusNotFound, "profile not found")
		return
	}

	imageURL := profileResp.Profile.ProfileImageUrl
	if imageURL == "" {
		log.Printf("[INFO] No profile picture set for userID=%s", userID)
		response.JSON(w, http.StatusOK, map[string]interface{}{
			"success":        true,
			"message":        "No profile picture set",
			"profile_image":  "",
		})
		return
	}

	log.Printf("[INFO] Successfully retrieved profile picture for userID=%s", userID)
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"message":        "Profile picture retrieved successfully",
		"profile_image":  imageURL,
	})
}


func (h *AuthHandler) DeleteProfilePicture(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		log.Println("[ERROR] Unauthorized delete attempt, missing userID in context")
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	log.Printf("[INFO] Starting profile picture deletion for userID=%s", userID)

	// Build file path (same logic as upload)
	uploadDir := "/app/uploads/profile_pictures"
	filename := fmt.Sprintf("%s.jpg", userID) // stored as JPEG
	filePath := filepath.Join(uploadDir, filename)

	// Remove profile picture in DB first (set to empty string)
	_, err := h.accountClient.Client.UpdateProfilePicture(
		context.Background(),
		&accountclient.UpdateProfilePictureRequest{
			UserId:   userID,
			ImageUrl: "",
		},
	)
	if err != nil {
		log.Printf("[ERROR] Failed to clear profile picture in account service for userID=%s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "failed to clear profile picture in account")
		return
	}
	log.Printf("[INFO] Profile picture cleared in account service for userID=%s", userID)

	// Delete file from disk if exists
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			log.Printf("[INFO] Profile picture file not found for userID=%s, nothing to delete", userID)
		} else {
			log.Printf("[ERROR] Failed to delete profile picture file for userID=%s: %v", userID, err)
			response.Error(w, http.StatusInternalServerError, "failed to delete profile picture file")
			return
		}
	} else {
		log.Printf("[INFO] Successfully deleted profile picture file for userID=%s", userID)
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Profile picture deleted successfully",
	})
}

//currentRole, ok2 = r.Context().Value(middleware.ContextRole).(string)


func (h *AuthHandler) HandleUpdateNationality(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// --- Get user ID and session type from context ---
	userIDVal := ctx.Value(middleware.ContextUserID)
	sessionTypeVal := ctx.Value(middleware.ContextSessionType)
	deviceIDVal := ctx.Value(middleware.ContextDeviceID)
	currentRoleVal := ctx.Value(middleware.ContextRole)

	userID, ok1 := userIDVal.(string)
	sessionType, ok2 := sessionTypeVal.(string)
	deviceID, ok3 := deviceIDVal.(string)
	currentRole, ok4 := currentRoleVal.(string)

	if !ok1 || userID == "" || !ok2 || !ok3 || !ok4 {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// --- Parse request body ---
	var req struct {
		Nationality string `json:"nationality"` // ISO2 code
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request")
		return
	}
	if req.Nationality == "" {
		response.Error(w, http.StatusBadRequest, "Nationality must be provided")
		return
	}

	// --- Check if nationality already exists ---
	natResp, err := h.accountClient.Client.GetUserNationality(ctx, &accountclient.GetUserNationalityRequest{
		UserId: userID,
	})
	if err != nil {
		log.Printf("GetUserNationality failed for user %s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "Failed to check nationality")
		return
	}
	if natResp.HasNationality {
		// Already set → inform user to contact support if they want to update
		response.JSON(w, http.StatusConflict, map[string]interface{}{
			"message": "Nationality is already set. Please contact support if you want to update it.",
		})
		return
	}

	// --- Update nationality ---
	updateResp, err := h.accountClient.Client.UpdateUserNationality(ctx, &accountclient.UpdateUserNationalityRequest{
		UserId:      userID,
		Nationality: req.Nationality,
	})
	if err != nil {
		log.Printf("UpdateUserNationality failed for user %s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "Failed to update nationality")
		return
	}
	if !updateResp.Success {
		errMsg := updateResp.Error
		if errMsg == "" {
			errMsg = "Unknown error"
		}
		response.Error(w, http.StatusInternalServerError, errMsg)
		return
	}

	// --- Upgrade role if currentRole is "any" ---
	if currentRole == "any" {
		log.Printf("Upgrading role for user %s from 'any' to 'kyc_unverified'", userID)
		err := h.handleRoleUpgrade(ctx, userID,"kyc_unverified")
		if err != nil {
			log.Printf("Role upgrade failed for user %s: %v", userID, err)
			// Optional: still proceed even if role upgrade fails
		}
	}

	// --- Create new general session if session was temp ---
	resp := map[string]interface{}{
		"message": "Nationality updated successfully",
	}
	if sessionType == "temp" {
		session, err := h.createSessionHelper(
			ctx,
			userID,
			false, // isTemp
			false, // isSingleUse
			"general",
			nil,
			&deviceID, nil, nil, r,
		)
		if err != nil {
			log.Printf("[HandleUpdateNationality] Failed to create new session for user %s: %v", userID, err)
			resp["warning"] = "Nationality updated but session creation failed. Please re-login."
		} else {
			resp["token"] = session.AuthToken
			resp["device"] = session.DeviceID
		}

		// Delete old token in background
		h.logoutSessionBg(ctx)
	}


	// --- Final response ---
	response.JSON(w, http.StatusOK, resp)
}



func (h *AuthHandler) handleRoleUpgrade(ctx context.Context, userID,newRole string)error{
	return h.urbacservice.AssignRoleByName(ctx, userID, newRole, 0)
}


func (h *AuthHandler) HandleGetPreferences(w http.ResponseWriter, r *http.Request) {
	// --- Extract user ID from context ---
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	ctx := r.Context()

	// --- Call account service to fetch preferences ---
	prefResp, err := h.accountClient.Client.GetPreferences(ctx, &accountclient.GetPreferencesRequest{
		UserId: userID,
	})
	if err != nil {
		log.Printf("GetPreferences failed for user %s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "Failed to fetch preferences")
		return
	}

	// --- Return preferences in response ---
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"preferences": prefResp.Preferences,
	})
}

func (h *AuthHandler) HandleUpdatePreferences(w http.ResponseWriter, r *http.Request) {
	// --- Extract user ID from context ---
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	ctx := r.Context()

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
	_, err := h.accountClient.Client.UpdatePreferences(ctx, &accountclient.UpdatePreferencesRequest{
		UserId:      userID,
		Preferences: prefs,
	})
	if err != nil {
		log.Printf("UpdatePreferences failed for user %s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "Failed to update preferences")
		return
	}

	// --- Success response ---
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "Preferences updated successfully",
	})
}

func (h *AuthHandler) HandleGetEmailVerificationStatus(w http.ResponseWriter, r *http.Request) {
	// --- Extract user ID from context ---
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	ctx := r.Context()

	// --- Call usecase ---
	isVerified, err := h.uc.GetEmailVerificationStatus(ctx, userID)
	if err != nil {
		if errors.Is(err, xerrors.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "User not found")
			return
		}
		log.Printf("GetEmailVerificationStatus failed for user %s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "Failed to fetch email verification status")
		return
	}

	// --- Return response ---
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"email_verified": isVerified,
	})
}


func (h *AuthHandler) HandleGetPhoneVerificationStatus(w http.ResponseWriter, r *http.Request) {
	// --- Extract user ID from context ---
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	ctx := r.Context()

	// --- Call usecase ---
	isVerified, err := h.uc.GetPhoneVerificationStatus(ctx, userID)
	if err != nil {
		if errors.Is(err, xerrors.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "User not found")
			return
		}
		log.Printf("GetPhoneVerificationStatus failed for user %s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "Failed to fetch phone verification status")
		return
	}

	// --- Return response ---
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"phone_verified": isVerified,
	})
}


func (h *AuthHandler) ensureNationality(ctx context.Context, userID string) (string, string) {
	// --- Extract user ID from context ---
	natResp, err := h.accountClient.Client.GetUserNationality(ctx, &accountclient.GetUserNationalityRequest{
		UserId: userID,
	})
	if err != nil {
		log.Printf("GetUserNationality failed for user %s: %v", userID, err)
		return "", ""
	}
	if natResp.HasNationality {
		return  "", natResp.Nationality
	} else {
		return "set_nationality",""
	}
}