package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"kyc-service/internal/service"
	"x/shared/auth/middleware"
	"x/shared/response"
	emailclient "x/shared/email"
	"x/shared/genproto/emailpb"


	"github.com/go-chi/chi"
)

type KYCHandler struct {
	service *service.KYCService
	emailClient *emailclient.EmailClient
}

func NewKYCHandler(s *service.KYCService, emailClient *emailclient.EmailClient) *KYCHandler {
	return &KYCHandler{service: s, emailClient: emailClient,}
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
	if idNumber == "" || documentType == "" || dateOfBirth == "" {
		log.Printf("[WARN] Missing required KYC fields for userID=%s", userID)
		response.Error(w, http.StatusBadRequest, "missing required fields")
		return
	}

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
	baseURL := "http://localhost:50057/uploads/kyc_docs"
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
	if h.emailClient == nil {
		return
	}
	if recipientEmail == "" {
		return
	}

	go func(uid, email string) {
		subject := "Your KYC submission has been received"
		body := `
			<!DOCTYPE html>
			<html><head><meta charset="UTF-8"><title>KYC Submitted</title></head>
			<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
				<div style="max-width: 600px; background-color: #ffffff; padding: 20px; border-radius: 8px; box-shadow: 0px 2px 5px rgba(0,0,0,0.1);">
					<h2 style="color: #2E86C1;">KYC Submission Received</h2>
					<p style="font-size: 16px; color: #333;">
						Hello,<br><br>
						We have successfully received your KYC documents. 
						Our compliance team is reviewing your submission, and you’ll be notified once the review is complete.
					</p>
					<p style="margin-top: 20px; font-size: 14px; color: #999999;">
						Thank you for verifying your account,<br>
						<strong>Pxyz Team</strong>
					</p>
				</div>
			</body>
			</html>`

		_, err := h.emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
			UserId:         uid,
			RecipientEmail: email,
			Subject:        subject,
			Body:           body,
			Type:           "kyc_submitted_pending",
		})
		if err != nil {
			log.Printf("[WARN] failed to send KYC submission email to %s: %v", email, err)
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

	status, err := h.service.GetStatus(r.Context(), userID)
	if err != nil {
		log.Printf("[ERROR] Failed to get KYC status for userID=%s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "failed to retrieve KYC status")
		return
	}

	log.Printf("[INFO] Successfully retrieved KYC status for userID=%s", userID)

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"kyc_status": status,
		"message":    "KYC status retrieved successfully",
	})
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
		response.Error(w, http.StatusInternalServerError,"failed to review KYC submission",)
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