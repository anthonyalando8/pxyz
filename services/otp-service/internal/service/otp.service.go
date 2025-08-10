package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"x/shared/genproto/emailpb" // adjust import path to your generated pb
	"google.golang.org/grpc"
	"x/shared/utils/id"
	"otp-service/internal/repository"
	"otp-service/internal/rate"
)

type OTPService struct {
	repo    *repository.OTPRepo
	limiter *rate.Limiter
	sf      *id.Snowflake
	emailClient emailpb.EmailServiceClient
	emailConn *grpc.ClientConn
	ttl     time.Duration
}

func NewOTPService(repo *repository.OTPRepo, limiter *rate.Limiter, sf *id.Snowflake, emailAddr string, ttl time.Duration) (*OTPService, error) {
	var conn *grpc.ClientConn
	if emailAddr != "" {
		c, err := grpc.Dial(emailAddr, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		conn = c
	}
	var emailClient emailpb.EmailServiceClient
	if conn != nil {
		emailClient = emailpb.NewEmailServiceClient(conn)
	}
	return &OTPService{
		repo: repo, limiter: limiter, sf: sf, emailClient: emailClient, emailConn: conn, ttl: ttl,
	}, nil
}

func (s *OTPService) Close() { if s.emailConn != nil { s.emailConn.Close() } }

func (s *OTPService) GenerateOTP(ctx context.Context, userID string, purpose, channel, recipient string) error {
	uid := userID
	if err := s.limiter.CanRequest(ctx, uid); err != nil {
		return err
	}

	// generate 6-digit code
	code := randomCode(6)
	now := time.Now()
	o := &repository.OTP{
		ID: s.sf.Generate(),
		UserID: userID,
		Code: code,
		Channel: channel,
		Purpose: purpose,
		IssuedAt: now,
		ValidUntil: now.Add(s.ttl),
		IsVerified: false,
		IsActive: true,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, o); err != nil {
		return err
	}

	// call email service if channel == "email"
	if channel == "email" && s.emailClient != nil {
		sub := fmt.Sprintf("Your OTP code (%s)", purpose)
		body := fmt.Sprintf("Your OTP code is %s. It will expire at %s", code, o.ValidUntil.Format(time.RFC3339))
		_, err := s.emailClient.SendEmail(ctx, &emailpb.SendEmailRequest{
			UserId: userID,
			RecipientEmail: recipient,
			Subject: sub,
			Body: body,
			Type: "otp",
		})
		if err != nil {
			// we don't fail hard if email fails; record is persisted
			return err
		}
	}
	return nil
}

func (s *OTPService) VerifyOTP(ctx context.Context, userID int64, purpose, code string) (bool, error) {
	ok, err := s.repo.VerifyAndInvalidate(ctx, userID, purpose, code)
	return ok, err
}

func randomCode(digits int) string {
	max := 1
	for i := 0; i < digits; i++ { max *= 10 }
	nBig := make([]byte, 8)
	_, _ = rand.Read(nBig)
	n := int(nBig[0])
	v := n % max
	return fmt.Sprintf("%0*d", digits, v)
}
