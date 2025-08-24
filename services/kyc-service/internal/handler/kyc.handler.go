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

	"github.com/go-chi/chi"
)

type KYCHandler struct {
	service *service.KYCService
}

func NewKYCHandler(s *service.KYCService) *KYCHandler {
	return &KYCHandler{service: s}
}

// UploadKYC handles uploading front and back ID images + KYC submission.
func (h *KYCHandler) UploadKYC(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from middleware context
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		log.Println("[ERROR] Unauthorized KYC upload attempt: missing userID in context")
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	

	// Parse form fields
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		log.Printf("[ERROR] Failed to parse multipart form: %v", err)
		response.Error(w, http.StatusBadRequest, "failed to parse form data")
		return
	}

	idNumber := r.FormValue("id_number")
	nationality := r.FormValue("nationality")
	documentType := r.FormValue("document_type")
	if idNumber == "" || nationality == "" {
		response.Error(w, http.StatusBadRequest, "missing required fields")
		return
	}

	// File uploads: front and back
	frontFile, frontHeader, err := r.FormFile("document_front")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "missing document front file")
		return
	}
	defer frontFile.Close()

	backFile, backHeader, err := r.FormFile("document_back")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "missing document back file")
		return
	}
	defer backFile.Close()

	// Upload dir
	uploadDir := "/app/uploads/kyc_docs"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("[ERROR] Failed to create upload dir %s: %v", uploadDir, err)
		response.Error(w, http.StatusInternalServerError, "failed to prepare upload dir")
		return
	}

	// Save files
	frontFilename := fmt.Sprintf("%s_front%s", userID, filepath.Ext(frontHeader.Filename))
	backFilename := fmt.Sprintf("%s_back%s", userID, filepath.Ext(backHeader.Filename))
	frontPath := filepath.Join(uploadDir, frontFilename)
	backPath := filepath.Join(uploadDir, backFilename)

	if err := saveFile(frontFile, frontPath); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to save front file")
		return
	}
	if err := saveFile(backFile, backPath); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to save back file")
		return
	}

	// Generate URLs (example, adjust base URL accordingly)
	frontURL := fmt.Sprintf("http://localhost:50057/uploads/kyc_docs/%s", frontFilename)
	backURL := fmt.Sprintf("http://localhost:50057/uploads/kyc_docs/%s", backFilename)

	// Call service
	err = h.service.Submit(
		context.Background(),
		userID, idNumber, nationality, documentType,
		frontURL, backURL,
	)
	if err != nil {
		log.Printf("[ERROR] Failed to submit KYC for userID=%s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "failed to submit kyc")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"message":       "KYC submitted successfully",
		"front_doc_url": frontURL,
		"back_doc_url":  backURL,
		"status":        "pending",
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


// --- NEW: Get status ---
func (h *KYCHandler) GetKYCStatus(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")


	status, err := h.service.GetStatus(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(status)
}

// --- NEW: Review submission (approve/reject) ---
func (h *KYCHandler) ReviewKYC(w http.ResponseWriter, r *http.Request) {
	kycID := chi.URLParam(r, "kycID")

	var req service.ReviewKYCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	req.KYCID = kycID

	err := h.service.Review(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- NEW: Get audit logs ---
func (h *KYCHandler) GetKYCAuditLogs(w http.ResponseWriter, r *http.Request) {
	kycID := chi.URLParam(r, "kycID")

	logs, err := h.service.GetAuditLogs(r.Context(), kycID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(logs)
}