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

	// Log to DB
	_ = h.repo.LogEmail(ctx, repository.EmailLog{
		ID: h.sf.Generate(),
		UserID:        req.UserId,
		Subject:       req.Subject,
		RecipientEmail: req.RecipientEmail,
		Type:          req.Type,
		Status:        status,
		SentAt:        time.Now(),
	})

	return &pb.SendEmailResponse{
		Success:       err == nil,
		ErrorMessage:  func() string { if err != nil { return err.Error() } else { return "" } }(),
	}, nil
}
