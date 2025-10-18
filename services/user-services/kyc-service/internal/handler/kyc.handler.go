package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"x/shared/utils/errors"

	"kyc-service/internal/domain"
	"kyc-service/internal/service"
	"x/shared/auth/middleware"
	emailclient "x/shared/email"
	"x/shared/genproto/emailpb"
	notificationpb "x/shared/genproto/shared/notificationpb"
	notificationclient "x/shared/notification" // ✅ added
	"x/shared/response"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/go-chi/chi"
)

type KYCHandler struct {
	service     *service.KYCService
	emailClient *emailclient.EmailClient
	notificationClient *notificationclient.NotificationService // ✅ added

}

func NewKYCHandler(s *service.KYCService, emailClient *emailclient.EmailClient, notificationClient *notificationclient.NotificationService) *KYCHandler {
	return &KYCHandler{service: s, emailClient: emailClient, notificationClient: notificationClient,}
}

// UploadKYC handles uploading front and back ID images + face photo + KYC submission.
func (h *KYCHandler) UploadKYC(w http.ResponseWriter, r *http.Request) {
	// Extract user ID
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		log.Println("[ERROR] Unauthorized KYC upload attempt: missing userID in context")
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	log.Printf("[INFO] Starting KYC upload for userID=%s", userID)

	// Parse form data
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		log.Printf("[ERROR] Failed to parse multipart form: %v", err)
		response.Error(w, http.StatusBadRequest, "failed to parse form data")
		return
	}

	idNumber := r.FormValue("id_number")
	documentType := r.FormValue("document_type")
	dateOfBirth := r.FormValue("date_of_birth")
	agreeTerms := r.FormValue("agree_terms") // checkbox from frontend

	// Check required fields
	if idNumber == "" || documentType == "" || dateOfBirth == "" {
		log.Printf("[WARN] Missing required KYC fields for userID=%s", userID)
		response.Error(w, http.StatusBadRequest, "missing required fields")
		return
	}

	// Check that terms were agreed
	if agreeTerms != "true" && agreeTerms != "on" {
		log.Printf("[WARN] User %s did not agree to terms", userID)
		response.Error(w, http.StatusBadRequest, "terms must be accepted")
		return
	}

	// Proceed with KYC processing
	log.Printf("[INFO] KYC fields valid for userID=%s", userID)


	// Get uploaded files
	frontFile, frontHeader, err := r.FormFile("document_front")
	if err != nil {
		log.Printf("[ERROR] Missing document_front for userID=%s", userID)
		response.Error(w, http.StatusBadRequest, "missing document front file")
		return
	}
	defer frontFile.Close()

	backFile, backHeader, err := r.FormFile("document_back")
	if err != nil {
		log.Printf("[ERROR] Missing document_back for userID=%s", userID)
		response.Error(w, http.StatusBadRequest, "missing document back file")
		return
	}
	defer backFile.Close()

	faceFile, faceHeader, err := r.FormFile("face_photo")
	if err != nil {
		log.Printf("[ERROR] Missing face_photo for userID=%s", userID)
		response.Error(w, http.StatusBadRequest, "missing face photo file")
		return
	}
	defer faceFile.Close()

	// Upload dir per user
	uploadDir := filepath.Join("/app/uploads/kyc_docs", userID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("[ERROR] Failed to create upload dir %s: %v", uploadDir, err)
		response.Error(w, http.StatusInternalServerError, "failed to prepare upload dir")
		return
	}
	log.Printf("[DEBUG] Upload directory prepared: %s", uploadDir)

	// Build file paths
	frontFilename := "front" + filepath.Ext(frontHeader.Filename)
	backFilename := "back" + filepath.Ext(backHeader.Filename)
	faceFilename := "face" + filepath.Ext(faceHeader.Filename)

	frontPath := filepath.Join(uploadDir, frontFilename)
	backPath := filepath.Join(uploadDir, backFilename)
	facePath := filepath.Join(uploadDir, faceFilename)

	// Save files
	if err := saveFile(frontFile, frontPath); err != nil {
		log.Printf("[ERROR] Failed to save front file for userID=%s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "failed to save front file")
		return
	}
	log.Printf("[INFO] Saved front document -> %s", frontPath)

	if err := saveFile(backFile, backPath); err != nil {
		log.Printf("[ERROR] Failed to save back file for userID=%s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "failed to save back file")
		return
	}
	log.Printf("[INFO] Saved back document -> %s", backPath)

	if err := saveFile(faceFile, facePath); err != nil {
		log.Printf("[ERROR] Failed to save face photo for userID=%s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "failed to save face file")
		return
	}
	log.Printf("[INFO] Saved face photo -> %s", facePath)

	// Generate URLs (example: adjust base URL to match your static file server)
	baseURL := "/kyc/uploads/kyc_docs"
	frontURL := fmt.Sprintf("%s/%s/%s", baseURL, userID, frontFilename)
	backURL := fmt.Sprintf("%s/%s/%s", baseURL, userID, backFilename)
	faceURL := fmt.Sprintf("%s/%s/%s", baseURL, userID, faceFilename)

	// Call service
	err = h.service.Submit(
		context.Background(),
		userID, idNumber, documentType,
		frontURL, backURL, faceURL, dateOfBirth,
	)
	if err != nil {
		log.Printf("[ERROR] Failed to submit KYC for userID=%s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "failed to submit kyc")
		return
	}
	log.Printf("[INFO] KYC submission stored successfully for userID=%s", userID)

	h.sendKYCSubmissionNotification(userID, "")

	// Response
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"message":        "KYC submitted successfully",
		"front_doc_url":  frontURL,
		"back_doc_url":   backURL,
		"face_photo_url": faceURL,
		"status":         "pending",
	})
}

func saveFile(src io.Reader, path string) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}

func (h *KYCHandler) sendKYCSubmissionNotification(userID, recipientEmail string) {
	if h.notificationClient == nil{
		return
	}

	go func(uid, email string) {
		ctx := context.Background() // background context for async processing

		_, err := h.notificationClient.Client.CreateNotification(ctx, &notificationpb.CreateNotificationsRequest{
			Notifications: []*notificationpb.Notification{
				{
					RequestId:      uuid.New().String(),
					OwnerType:      "user",
					OwnerId:        uid,
					EventType:      "KYC_SUBMITTED",
					Title: "KYC Documents Submitted",
					Body: "Your KYC documents have been submitted awaiting review.",
					ChannelHint:    []string{"email"},
					Payload: func() *structpb.Struct {
						s, _ := structpb.NewStruct(map[string]interface{}{})
						return s
					}(),
					VisibleInApp:   false,
					RecipientEmail: email,
					Priority:       "high",
					Status:         "pending",
				},
			},
		})
		if err != nil {
			log.Printf("[WARN] failed to send KYC submission email to %s: %v", email, err)
		} else {
			log.Printf("Successfully queued KYC submission notification | Recipient=%s", email)
		}
	}(userID, recipientEmail)
}


func (h *KYCHandler) sendKYCReviewResult(userID, recipientEmail, status string) {
	subject := "Your KYC review is complete"
	body := fmt.Sprintf(`
		<!DOCTYPE html>
		<html><head><meta charset="UTF-8"><title>KYC Review</title></head>
		<body>
			<p>Hello,</p>
			<p>Your KYC review is now complete. Status: <strong>%s</strong></p>
			<p>Thank you,<br>Pxyz Team</p>
		</body>
		</html>`, status)

	_, err := h.emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
		UserId:         userID,
		RecipientEmail: recipientEmail,
		Subject:        subject,
		Body:           body,
		Type:           "kyc_review_result",
	})
	if err != nil {
		log.Printf("[WARN] failed to send KYC review result email to %s: %v", recipientEmail, err)
	}
}

func (h *KYCHandler) GetKYCStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		log.Println("[ERROR] Missing userID in KYC status request")
		response.Error(w, http.StatusBadRequest, "missing userID")
		return
	}
	log.Printf("[INFO] Retrieving KYC status for userID=%s", userID)

	kycSubmission, err := h.service.GetStatus(r.Context(), userID)
	if err != nil {
		if errors.Is(err, xerrors.ErrNotFound) {
			log.Printf("[INFO] No KYC submission found for userID=%s", userID)
			response.JSON(w, http.StatusOK, map[string]interface{}{
				"submitted":  false,
				"kyc_status": "not_submitted",
				"message":    "User has not submitted any KYC documents yet",
			})
			return
		}

		log.Printf("[ERROR] Failed to get KYC status for userID=%s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "failed to retrieve KYC status")
		return
	}

	log.Printf("[INFO] Successfully retrieved KYC status for userID=%s", userID)

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"submitted":  true,
		"kyc_status": kycSubmission.Status,
		"message":    "KYC status retrieved successfully",
	})
}

func (h *KYCHandler) GetKYCSubmission(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		log.Println("[ERROR] Missing userID in KYC submission request")
		response.Error(w, http.StatusBadRequest, "missing userID")
		return
	}

	log.Printf("[INFO] Retrieving KYC submission for userID=%s", userID)

	kycSubmission, err := h.service.GetStatus(r.Context(), userID)
	if err != nil {
		if errors.Is(err, xerrors.ErrNotFound) {
			log.Printf("[INFO] No KYC submission found for userID=%s", userID)
			response.Error(w, http.StatusNotFound, "no KYC submission found")
			return
		}

		log.Printf("[ERROR] Failed to get KYC submission for userID=%s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "failed to retrieve KYC submission")
		return
	}

	// Map DB model to response struct (exclude UpdatedAt)
	resp := domain.KYCSubmissionResponse{
		ID:               kycSubmission.ID,
		UserID:           kycSubmission.UserID,
		IDNumber:         kycSubmission.IDNumber,
		DocumentType:     kycSubmission.DocumentType,
		DocumentFrontURL: kycSubmission.DocumentFrontURL,
		DocumentBackURL:  kycSubmission.DocumentBackURL,
		FacePhotoURL:     kycSubmission.FacePhotoURL,
		Status:           kycSubmission.Status,
		RejectionReason:  kycSubmission.RejectionReason,
	}

	// Convert non-zero times to pointers (so omitempty works correctly)
	if !kycSubmission.DateOfBirth.IsZero() {
		resp.DateOfBirth = &kycSubmission.DateOfBirth
	}
	if !kycSubmission.SubmittedAt.IsZero() {
		resp.SubmittedAt = &kycSubmission.SubmittedAt
	}
	if kycSubmission.ReviewedAt != nil && !kycSubmission.ReviewedAt.IsZero() {
		resp.ReviewedAt = kycSubmission.ReviewedAt
	}

	log.Printf("[INFO] Successfully retrieved KYC submission for userID=%s", userID)
	response.JSON(w, http.StatusOK, resp)
}

// --- NEW: Review submission (approve/reject) ---
func (h *KYCHandler) ReviewKYC(w http.ResponseWriter, r *http.Request) {
	kycID := chi.URLParam(r, "kycID")
	if kycID == "" {
		log.Println("[KYC][ERROR] Missing kycID in review request")
		response.Error(w, http.StatusBadRequest, "missing kycID")
		return
	}
	log.Printf("[KYC][INFO] Starting review for kycID=%s", kycID)

	var req service.ReviewKYCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[KYC][ERROR] Invalid review request body for kycID=%s: %v", kycID, err)
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.KYCID = kycID
	log.Printf("[KYC][DEBUG] Review request parsed for kycID=%s: status=%s reason=%q",
		kycID, req.Decision, req.RejectionNote)

	// Call service
	if err := h.service.Review(r.Context(), &req); err != nil {
		log.Printf("[KYC][ERROR] Failed to review KYC submission kycID=%s: %v", kycID, err)
		response.Error(w, http.StatusInternalServerError, "failed to review KYC submission")
		return
	}

	log.Printf("[KYC][INFO] Successfully reviewed KYC submission kycID=%s with status=%s",
		kycID, req.Decision)

	w.WriteHeader(http.StatusNoContent)
}

// --- NEW: Get audit logs ---
func (h *KYCHandler) GetKYCAuditLogs(w http.ResponseWriter, r *http.Request) {
	kycID := chi.URLParam(r, "kycID")

	logs, err := h.service.GetAuditLogs(r.Context(), kycID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	json.NewEncoder(w).Encode(logs)
}
