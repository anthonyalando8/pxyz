package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"x/shared/auth/middleware"
	accountclient "x/shared/genproto/accountpb"
	"x/shared/response"
	"x/shared/utils/image"
	stdimage "image"
	"strings"

	_ "image/gif" // register gif
	_ "image/jpeg"
	_ "image/png"

)

func (h *AuthHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	requestedUserID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || requestedUserID == "" {
		log.Printf("[WARN] Unauthorized access attempt from %s", r.RemoteAddr)
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log.Printf("[INFO] Fetching profile for user_id=%s", requestedUserID)

	// Get user (from user table)
	user, err := h.uc.GetProfile(r.Context(), requestedUserID)
	if err != nil {
		log.Printf("[ERROR] Failed to retrieve user record user_id=%s error=%v", requestedUserID, err)
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve user")
		return
	}
	log.Printf("[DEBUG] Retrieved user record user_id=%s email=%s", requestedUserID, safeString(user.Email))

	// Get profile (from account-service gRPC)
	profileResp, err := h.accountClient.Client.GetUserProfile(r.Context(), &accountclient.GetUserProfileRequest{
		UserId: requestedUserID,
	})
	if err != nil {
		log.Printf("[ERROR] Failed to fetch profile from account-service user_id=%s error=%v", requestedUserID, err)
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve user profile")
		return
	}
	if profileResp == nil || profileResp.Profile == nil {
		log.Printf("[WARN] Profile not found in account-service user_id=%s", requestedUserID)
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve user profile")
		return
	}
	profile := profileResp.Profile
	log.Printf("[DEBUG] Retrieved profile from account-service user_id=%s", requestedUserID)

	// Merge response
	resp := map[string]interface{}{
		"user_id":           requestedUserID,
		"email":             safeString(user.Email),
		"phone":             safeString(user.Phone),
		"is_email_verified": user.IsEmailVerified,
		"is_phone_verified": user.IsPhoneVerified,
		"account_type":      user.AccountType,
		"account_status":    user.AccountStatus,
		"created_at":        user.CreatedAt,
		"updated_at":        user.UpdatedAt,

		// Extended profile
		"first_name":    profile.FirstName,
		"last_name":     profile.LastName,
		"bio":           profile.Bio,
		"gender":        profile.Gender,
		"date_of_birth": profile.DateOfBirth,
		"profile_image": profile.ProfileImageUrl,
	}
	var address map[string]interface{}
	if err := json.Unmarshal([]byte(profile.AddressJson), &address); err == nil {
		resp["address"] = address
	} else {
		resp["address"] = profile.AddressJson // fallback to string
	}

	log.Printf("[INFO] Successfully fetched profile user_id=%s", requestedUserID)
	response.JSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) HandleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	requestedUserID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || requestedUserID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req UpdateProfileRequest
	if err := decodeRequestBody(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Prepare gRPC request
	updateReq := &accountclient.UpdateProfileRequest{
		UserId: requestedUserID,
	}

	// Only set fields if non-nil
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
		if addrBytes, err := json.Marshal(req.Address); err == nil {
			updateReq.AddressJson = string(addrBytes)
		} else {
			response.Error(w, http.StatusBadRequest, "Invalid address format")
			return
		}
	}

	// Call account-service gRPC
	_, err := h.accountClient.Client.UpdateProfile(r.Context(), updateReq)
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
	imageURL := fmt.Sprintf("http://localhost:50051/uploads/profile_pictures/%s", filename)

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


