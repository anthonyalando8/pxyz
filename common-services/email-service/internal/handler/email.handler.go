package handler

import (
	"context"
	"log"
	"time"

	pb "x/shared/genproto/emailpb"
	"email-service/internal/repository"
	"email-service/internal/service"
	"x/shared/utils/id"
)

type EmailHandler struct {
	pb.UnimplementedEmailServiceServer
	emailSvc *service.EmailSender
	repo     *repository.EmailLogRepo
	sf          *id.Snowflake
}

func NewEmailHandler(emailSvc *service.EmailSender, repo *repository.EmailLogRepo, sf *id.Snowflake) *EmailHandler {
	return &EmailHandler{emailSvc: emailSvc, repo: repo, sf: sf}
}

func (h *EmailHandler) SendEmail(ctx context.Context, req *pb.SendEmailRequest) (*pb.SendEmailResponse, error) {
	err := h.emailSvc.Send(req.RecipientEmail, req.Subject, req.Body)

	status := "sent"
	if err != nil {
		status = "failed"
		log.Printf("Error sending email: %v", err)
	}

	// Fire-and-forget DB log
	go func(req *pb.SendEmailRequest, status string, err error) {
		// derive a fresh context with timeout to avoid leaks
		bgCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		logErr := h.repo.LogEmail(bgCtx, repository.EmailLog{
			ID:             h.sf.Generate(),
			UserID:         req.UserId,
			Subject:        req.Subject,
			RecipientEmail: req.RecipientEmail,
			Type:           req.Type,
			Status:         status,
			SentAt:         time.Now(),
		})
		if logErr != nil {
			// Only log, don’t bubble up
			log.Printf("failed to log email send: %v", logErr)
		}
	}(req, status, err)

	return &pb.SendEmailResponse{
		Success:      err == nil,
		ErrorMessage: func() string { if err != nil { return err.Error() } else { return "" } }(),
	}, nil
}

