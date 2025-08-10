package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"x/shared/genproto/emailpb"
	"x/shared/email"
	"x/shared/utils/id"

	"otp-service/internal/repository"
	"otp-service/internal/rate"
)

type OTPService struct {
	repo        *repository.OTPRepo
	limiter     *rate.Limiter
	sf          *id.Snowflake
	emailClient *emailclient.EmailClient
	ttl         time.Duration
}

func NewOTPService(
	repo *repository.OTPRepo,
	limiter *rate.Limiter,
	sf *id.Snowflake,
	emailClient *emailclient.EmailClient, // inject ready client
	ttl time.Duration,
) *OTPService {
	return &OTPService{
		repo:        repo,
		limiter:     limiter,
		sf:          sf,
		emailClient: emailClient,
		ttl:         ttl,
	}
}

func (s *OTPService) GenerateOTP(ctx context.Context, userID string, purpose, channel, recipient string) error {
	// rate limit check
	if err := s.limiter.CanRequest(ctx, userID); err != nil {
		return err
	}

	// generate 6-digit code
	code := randomCode(6)
	now := time.Now()
	o := &repository.OTP{
		ID:         s.sf.Generate(),
		UserID:     userID,
		Code:       code,
		Channel:    channel,
		Purpose:    purpose,
		IssuedAt:   now,
		ValidUntil: now.Add(s.ttl),
		IsVerified: false,
		IsActive:   true,
		UpdatedAt:  now,
	}

	if err := s.repo.Create(ctx, o); err != nil {
		return err
	}

	// Send email if needed
	if channel == "email" && s.emailClient != nil {
		subject := fmt.Sprintf("Your OTP code (%s)", purpose)
		body := fmt.Sprintf("Your OTP code is %s. It will expire at %s",
			code, o.ValidUntil.Format(time.RFC3339))

		_, err := s.emailClient.SendEmail(ctx, &emailpb.SendEmailRequest{
			UserId:         userID,
			RecipientEmail: recipient,
			Subject:        subject,
			Body:           body,
			Type:           "otp",
		})
		if err != nil {
			// Don't block OTP persistence if email fails
			return fmt.Errorf("failed to send email: %w", err)
		}
	}

	return nil
}

func (s *OTPService) VerifyOTP(ctx context.Context, userID int64, purpose, code string) (bool, error) {
	return s.repo.VerifyAndInvalidate(ctx, userID, purpose, code)
}

func randomCode(digits int) string {
	max := 1
	for i := 0; i < digits; i++ {
		max *= 10
	}
	nBig := make([]byte, 8)
	_, _ = rand.Read(nBig)
	n := int(nBig[0])
	v := n % max
	return fmt.Sprintf("%0*d", digits, v)
}
