// internal/handler/email_handler. go
package handler

import (
	"context"
	"time"

	pb "x/shared/genproto/emailpb"
	"email-service/internal/repository"
	"email-service/internal/service"
	"x/shared/utils/id"

	"go.uber.org/zap"
)

type EmailHandler struct {
	pb.UnimplementedEmailServiceServer
	emailSvc      *service.EmailSender
	repo          *repository.EmailLogRepo
	sf            *id.Snowflake
	logger        *zap.Logger
}

func NewEmailHandler(
	emailSvc *service.EmailSender,
	repo *repository.EmailLogRepo,
	sf *id.Snowflake,
	logger *zap.Logger,
) *EmailHandler {
	return &EmailHandler{
		emailSvc:      emailSvc,
		repo:          repo,
		sf:            sf,
		logger:        logger,
	}
}

// SendEmail sends an email using the improved email service
func (h *EmailHandler) SendEmail(ctx context.Context, req *pb.SendEmailRequest) (*pb.SendEmailResponse, error) {
	startTime := time.Now()
	emailID := h.sf.Generate()

	h.logger.Info("sending email",
		zap.String("email_id", emailID),
		zap.String("recipient", req.RecipientEmail),
		zap.String("subject", req.Subject),
		zap.String("type", req. Type),
		zap.String("user_id", req.UserId))

	// Build email message
	msg := service.EmailMessage{
		To:        req.RecipientEmail,
		Subject:   req.Subject,
		HTMLBody:  req.Body,
		PlainBody: service.HTMLToPlainText(req.Body), // ✅ Auto-generate plain text
		Category:  req.Type,
	}

	// Send email
	err := h.emailSvc.Send(msg)

	// Calculate send duration
	duration := time.Since(startTime)

	// Determine status
	status := "sent"
	var errorMessage string

	if err != nil {
		status = "failed"
		errorMessage = err.Error()

		// ✅ Log failure with full details
		h.logger.Error("email send failed",
			zap.String("email_id", emailID),
			zap.String("recipient", req.RecipientEmail),
			zap.String("subject", req.Subject),
			zap.String("type", req.Type),
			zap.String("user_id", req.UserId),
			zap.Duration("duration", duration),
			zap.Error(err))
	} else {
		// ✅ Log success
		h.logger.Info("email sent successfully",
			zap. String("email_id", emailID),
			zap.String("recipient", req.RecipientEmail),
			zap.String("subject", req.Subject),
			zap.String("type", req.Type),
			zap.String("user_id", req. UserId),
			zap.Duration("duration", duration))
	}

	// Fire-and-forget DB log
	go h.logEmailToDatabase(emailID, req, status, errorMessage, duration)

	return &pb.SendEmailResponse{
		Success:       err == nil,
		ErrorMessage: errorMessage,
	}, nil
}

// ============================================
// BACKGROUND LOGGING FUNCTIONS
// ============================================

// logEmailToDatabase logs email send to database
func (h *EmailHandler) logEmailToDatabase(emailID string, req *pb.SendEmailRequest, status, errorMessage string, duration time.Duration) {
	// Create fresh context with timeout
	bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	emailLog := repository.EmailLog{
		ID:             emailID,
		UserID:         req.UserId,
		Subject:        req.Subject,
		RecipientEmail: req.RecipientEmail,
		Type:            req.Type,
		Status:         status,
		ErrorMessage:   errorMessage,
		SentAt:         time.Now(),
		Duration:       duration,
	}

	if err := h.repo.LogEmail(bgCtx, emailLog); err != nil {
		h. logger.Error("failed to log email to database",
			zap. String("email_id", emailID),
			zap.String("recipient", req.RecipientEmail),
			zap.String("status", status),
			zap.Error(err))
	} else {
		h.logger. Debug("email logged to database",
			zap.String("email_id", emailID),
			zap.String("recipient", req.RecipientEmail),
			zap.String("status", status))
	}
}